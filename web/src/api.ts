import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

const healthSchema = z.object({
  status: z.literal('ok'),
  service: z.literal('pluginpocket'),
  version: z
    .string()
    .regex(/^[^\p{Cc}]*$/u)
    .trim()
    .min(1),
});

export const healthQuery = queryOptions({
  queryKey: ['health'],
  queryFn: async ({ signal }) => {
    const response = await fetch('/api/v1/health', {
      signal: AbortSignal.any([signal, AbortSignal.timeout(5_000)]),
      cache: 'no-store',
    });
    if (!response.ok) throw new Error('Health check failed');
    return healthSchema.parse(await response.json());
  },
  retry: false,
  networkMode: 'always',
  refetchOnReconnect: false,
  staleTime: 0,
  refetchOnWindowFocus: false,
});
