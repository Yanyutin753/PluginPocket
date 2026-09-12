import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';
import { errorText } from '../../i18n/errors';

export const userSchema = z.object({
  id: z.number().int(),
  username: z.string(),
  role: z.enum(['user', 'admin']),
  billing_role: z.string().optional(),
  balance: z.number().int(),
  enabled: z.boolean(),
});
export type User = z.infer<typeof userSchema>;
export const tokenSchema = z.object({
  id: z.number().int(),
  name: z.string(),
  prefix: z.string(),
  wallet_id: z.number().int(),
  created_at: z.iso.datetime({ offset: true }),
  revoked_at: z.iso.datetime({ offset: true }).nullable(),
  last_used_at: z.iso.datetime({ offset: true }).nullable(),
});
export const usageSchema = z.object({
  user_id: z.number().int().optional(),
  id: z.number().int(),
  tool: z.string(),
  cost: z.number().int(),
  status: z.enum(['pending', 'ok', 'error', 'recovered', 'denied']),
  duration_ms: z.number(),
  created_at: z.iso.datetime({ offset: true }),
  billing_role: z.string().optional(),
  multiplier_bp: z.number().int().optional(),
});
export const usageDetailSchema = z.object({
  item: usageSchema.extend({
    input_data: z.string().nullable(),
    output_data: z.string().nullable(),
    input_truncated: z.boolean(),
    output_truncated: z.boolean(),
  }),
});
export const pageSchema = <T extends z.ZodType>(item: T) =>
  z.object({ items: z.array(item), next_cursor: z.string() });
export const resultSchema = z.object({ user: userSchema });
const accountSchema = z.object({
  user: userSchema,
  summary: z.object({
    today_calls: z.number().int(),
    month_cost: z.number().int(),
    token_count: z.number().int(),
  }),
});
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
  ) {
    super(errorText(code, 'zh-CN'));
  }
}
async function fetchOnce(
  path: string,
  init: RequestInit = {},
): Promise<Response> {
  try {
    const response = await fetch(`/api/v1${path}`, {
      ...init,
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { 'Content-Type': 'application/json', ...init.headers },
      signal: AbortSignal.any([
        ...(init.signal ? [init.signal] : []),
        AbortSignal.timeout(10_000),
      ]),
    });
    if (!response.ok) {
      const parsed = z
        .object({ error: z.string() })
        .safeParse(await response.json().catch(() => null));
      throw new ApiError(
        response.status,
        parsed.success ? parsed.data.error : 'unknown',
      );
    }
    return response;
  } catch (error) {
    if (error instanceof ApiError) throw error;
    throw new ApiError(0, 'unknown');
  }
}
let refreshInFlight: Promise<void> | undefined;
let refreshGeneration = 0;
export async function fetchResponse(
  path: string,
  init: RequestInit = {},
): Promise<Response> {
  const generation = refreshGeneration;
  try {
    return await fetchOnce(path, init);
  } catch (error) {
    if (
      !(error instanceof ApiError) ||
      error.status !== 401 ||
      error.code !== 'unauthorized' ||
      path.startsWith('/auth/') ||
      init.signal?.aborted
    )
      throw error;
    if (generation === refreshGeneration) {
      refreshInFlight ??= fetchOnce('/auth/refresh', { method: 'POST' })
        .then(() => {
          refreshGeneration++;
        })
        .finally(() => {
          refreshInFlight = undefined;
        });
      await refreshInFlight;
    }
    if (init.signal?.aborted) throw new ApiError(0, 'unknown');
    return fetchOnce(path, init);
  }
}
export async function request<T>(
  path: string,
  schema: z.ZodType<T>,
  init: RequestInit = {},
): Promise<T> {
  const response = await fetchResponse(path, init);
  try {
    return schema.parse(response.status === 204 ? null : await response.json());
  } catch {
    throw new ApiError(0, 'unknown');
  }
}
export async function requestCSV(path: string) {
  const response = await fetchResponse(path);
  if (!response.headers.get('Content-Type')?.startsWith('text/csv'))
    throw new ApiError(0, 'unknown');
  try {
    const text = await response.text();
    const cursor = z
      .string()
      .regex(/^\d*$/)
      .parse(response.headers.get('X-Next-Cursor') ?? '');
    return { text, cursor };
  } catch {
    throw new ApiError(0, 'unknown');
  }
}
export const metaQuery = queryOptions({
  queryKey: ['meta'],
  queryFn: ({ signal }) =>
    request(
      '/meta',
      z.object({
        github: z.boolean(),
        email: z.boolean(),
        payments: z.boolean(),
      }),
      { signal },
    ),
  retry: false,
  staleTime: 60_000,
});
export const billingRoleSchema = z.object({
  name: z.string(),
  multiplier_bp: z.number().int().min(0),
  description: z.string(),
});
export type BillingRole = z.infer<typeof billingRoleSchema>;
export const billingRolesQuery = queryOptions({
  queryKey: ['/admin/billing-roles'],
  queryFn: ({ signal }) =>
    request(
      '/admin/billing-roles',
      z.object({ items: z.array(billingRoleSchema) }),
      { signal },
    ),
  retry: false,
  staleTime: 30_000,
});
// 基点倍率（10000 = ×1）转展示文案，去掉多余的尾零。
export function multiplierText(bp: number): string {
  const text = `${(bp / 10000).toFixed(2)}`
    .replace(/(\.\d*?)0+$/, '$1')
    .replace(/\.$/, '');
  return `×${text}`;
}
export const accountQuery = queryOptions({
  queryKey: ['account'],
  queryFn: ({ signal }) => request('/account/me', accountSchema, { signal }),
  retry: false,
  staleTime: 30_000,
});
export const publicSessionQuery = queryOptions({
  queryKey: ['public-session'],
  queryFn: async ({ signal }) => {
    try {
      return await request('/account/me', accountSchema, { signal });
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) return null;
      throw error;
    }
  },
  retry: false,
  staleTime: 0,
});
export const listOptions = <T extends z.ZodType>(
  path: string,
  schema: T,
  limit = 50,
) => ({
  queryKey: [path, { limit }],
  initialPageParam: '',
  queryFn: ({
    signal,
    pageParam,
  }: {
    signal: AbortSignal;
    pageParam: string;
  }) =>
    request(
      `${path}${path.includes('?') ? '&' : '?'}limit=${limit}&cursor=${encodeURIComponent(pageParam)}`,
      pageSchema(schema),
      { signal },
    ),
  getNextPageParam: (page: { next_cursor: string }) =>
    page.next_cursor || undefined,
  retry: false,
});
export const number = (value: number, locale = 'zh-CN') =>
  new Intl.NumberFormat(locale).format(value);
export const date = (value: string | null, locale = 'zh-CN') =>
  value
    ? new Intl.DateTimeFormat(locale, {
        dateStyle: 'medium',
        timeStyle: 'short',
      }).format(new Date(value))
    : locale === 'en'
      ? 'Never used'
      : '从未使用';
