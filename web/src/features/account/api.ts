import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

export const userSchema = z.object({
  id: z.number().int(),
  username: z.string(),
  role: z.enum(['user', 'admin']),
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
const messages: Record<string, string> = {
  settings_conflict: '其他管理员已更新配置，请重新加载后再编辑。',
  settings_unavailable: '系统配置暂不可用，请检查服务部署后重试。',
  settings_encryption_unavailable:
    '部署未配置加密主密钥，暂不能保存新的集成密钥。',
  payment_unavailable: '在线支付暂未配置，请使用兑换码或联系管理员补充额度。',
  rate_limited: '请求过于频繁，请稍后再试。',
  email_unavailable: '邮件服务暂时不可用，请稍后再试。',
  invalid_token: '验证链接无效或已过期，请重新发送验证邮件。',
  already_redeemed: '此兑换码已经使用，请核对额度流水。',
  invalid_json: 'JSON 格式不正确，请检查参数或连接配置后重试。',
  expired_token: '授权码已过期，请在设备上重新发起登录。',
  invalid_grant: '授权码无效或已使用，请在设备上重新发起登录。',
  seat_limit: '团队席位已满，请联系团队所有者。',
  invite_expired: '邀请码已过期，请联系团队所有者生成新邀请。',
  invite_used: '邀请码已经使用，请联系团队所有者生成新邀请。',
  already_member: '你已经是此团队成员。',
  team_full: '团队席位已满，请联系团队所有者调整席位。',
  seats_in_use: '席位数量不能少于现有成员人数。',
  cannot_disable_self: '不能停用当前登录账号。',
  last_admin: '需要保留至少一位启用的管理员。',
  invalid_device_code:
    '设备授权码无效、已使用或已过期，请在设备上重新发起登录。',
  last_owner: '最后一位团队所有者不能退出。',
  username_taken: '用户名已被使用，请换一个用户名。',
  invalid_credentials: '用户名或密码不正确，请重新输入。',
  invalid_request: '提交内容不符合要求，请检查后重试。',
  invalid_settlement: '结算策略无效：检查 JSON、业务码路径或正则。',
  invalid_settlement_script: '结算脚本语法有误，请修正后再保存。',
  forbidden: '没有权限执行此操作。',
  forbidden_origin: '请求来源未通过验证，请从本站重新打开页面。',
  insufficient_balance: '可用额度不足，请检查调整金额。',
  idempotency_conflict: '这次操作与已有记录冲突，请关闭表单并重新核对。',
  not_found: '记录已不存在，请刷新后重试。',
  unauthorized: '登录已过期，请重新登录。',
};
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
  ) {
    super(messages[code] ?? '暂时无法完成请求，请检查连接后重试。');
  }
}
export async function fetchResponse(
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
export const accountQuery = queryOptions({
  queryKey: ['account'],
  queryFn: ({ signal }) => request('/account/me', accountSchema, { signal }),
  retry: false,
  staleTime: 30_000,
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
