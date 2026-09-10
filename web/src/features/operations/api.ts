import { z } from 'zod';

const timestamp = z.iso.datetime({ offset: true });
export const toolSchema = z.object({
  id: z.number().int(),
  key: z.string(),
  name: z.string(),
  description: z.string(),
  kind: z.enum(['builtin', 'http', 'stdio']),
  enabled: z.boolean(),
  units_per_call: z.number().int(),
  input_schema: z.record(z.string(), z.unknown()),
  configured: z.boolean().optional(),
});
export type Tool = z.infer<typeof toolSchema>;
export const planSchema = z.object({
  id: z.number().int(),
  name: z.string(),
  credits: z.number().int(),
  price_cents: z.number().int(),
  currency: z.string(),
  enabled: z.boolean(),
  created_at: timestamp,
});
export type Plan = z.infer<typeof planSchema>;
export const codeSchema = z.object({
  id: z.number().int(),
  credits: z.number().int(),
  note: z.string(),
  created_at: timestamp,
  redeemed_at: timestamp.nullable(),
});
export const ledgerSchema = z.object({
  id: z.number().int(),
  delta: z.number().int(),
  kind: z.string(),
  note: z.string(),
  created_at: timestamp,
  balance_after: z.number().int().nullable(),
});
export const orderSchema = z.object({
  id: z.number().int(),
  plan_id: z.number().int(),
  credits: z.number().int(),
  price_cents: z.number().int(),
  currency: z.string(),
  status: z.string(),
  created_at: timestamp,
});
export const teamSchema = z.object({
  id: z.number().int(),
  name: z.string(),
  role: z.enum(['owner', 'member']),
  balance: z.number().int(),
  seat_limit: z.number().int(),
});
export type Team = z.infer<typeof teamSchema>;
export const memberSchema = z.object({
  id: z.number().int(),
  user_id: z.number().int(),
  username: z.string(),
  role: z.enum(['owner', 'member']),
});
export const price = (cents: number, currency: string, locale = 'zh-CN') =>
  `${new Intl.NumberFormat(locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(cents / 100)} ${currency}`;
