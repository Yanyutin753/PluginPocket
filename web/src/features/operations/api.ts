import { z } from 'zod';

export const publicPluginSchema = z.object({
  slug: z.string(),
  name: z.string(),
  description: z.string(),
  kind: z.enum(['mcp', 'skill', 'bundle']),
  version: z.string(),
  gateway: z.boolean(),
});
export const publicPluginsSchema = z.object({
  items: z.array(publicPluginSchema),
  origin: z.string(),
});
export const publicPluginDetailSchema = z.object({
  item: publicPluginSchema,
  origin: z.string(),
});

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
  settlement: z.record(z.string(), z.unknown()).optional(),
  configured: z.boolean().optional(),
  icon: z.string().optional(),
});
export type Tool = z.infer<typeof toolSchema>;
export const marketplaceItemSchema = z.object({
  id: z.number().int(),
  slug: z.string(),
  name: z.string(),
  description: z.string(),
  kind: z.enum(['mcp', 'skill', 'bundle']).default('mcp'),
  source: z.enum(['curated', 'github']),
  repo_url: z.string(),
  homepage: z.string(),
  transport: z.enum(['http', 'stdio', 'unknown', 'gateway']),
  version: z.string().default('1.0.0'),
  spec: z
    .object({
      source: z.enum(['inline', 'github']).optional(),
      repo: z.string().optional(),
      path: z.string().optional(),
      files: z.record(z.string(), z.string()).optional(),
      includes: z.array(z.string()).optional(),
    })
    .optional(),
  endpoint: z.string(),
  package: z.string(),
  stars: z.number().int(),
  synced_at: timestamp.nullable(),
  installed: z.boolean(),
});
export type MarketplaceItem = z.infer<typeof marketplaceItemSchema>;
export const upstreamToolSchema = z.object({
  name: z.string(),
  description: z.string(),
  input_schema: z.unknown(),
});
export type UpstreamTool = z.infer<typeof upstreamToolSchema>;
export const metadataOverrideSchema = z.object({
  remote_name: z.string(),
  description: z.string(),
  input_schema: z.unknown().optional(),
});
export type MetadataOverride = z.infer<typeof metadataOverrideSchema>;
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
