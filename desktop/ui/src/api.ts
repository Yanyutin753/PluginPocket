import { invoke } from '@tauri-apps/api/core';
import { z } from 'zod';

const clientKind = z.enum(['codex', 'claude', 'cursor']);
export type ClientKind = z.infer<typeof clientKind>;
const client = z.object({
  client: clientKind,
  detected: z.boolean(),
  configured: z.boolean(),
});
const clients = z.array(client);
const account = z.object({
  username: z.string().min(1),
  balance: z.number().int().nonnegative(),
  tools: z.array(z.string()),
});
const status = z.object({ account, clients });
const doctor = z.object({
  version: z.string(),
  authenticated: z.boolean(),
  clients,
});
export type Status = z.infer<typeof status>;
const installed = z.array(
  z.object({
    slug: z.string().min(1),
    kind: z.enum(['mcp', 'skill']),
    clients: z.array(clientKind).min(1),
    version: z.string().nullable(),
  }),
);
const logs = z.array(
  z.object({
    timestamp: z.number().int().nonnegative().max(8640000000000000),
    level: z.enum(['info', 'error']),
    action: z.string(),
    message: z.string(),
  }),
);
const diagnostics = z.object({
  checks: z.array(
    z.object({
      id: z.string(),
      label: z.string(),
      status: z.enum(['ok', 'warning', 'error']),
      detail: z.string(),
    }),
  ),
});
export type InstalledItem = z.infer<typeof installed>[number];
export type LogEntry = z.infer<typeof logs>[number];
export const clientLabels: Record<ClientKind, string> = {
  codex: 'Codex',
  claude: 'Claude Code',
  cursor: 'Cursor',
};
const command = (command: Record<string, unknown>) =>
  invoke<unknown>('local_command', { command });
export const api = {
  installed: async () =>
    installed.parse(await command({ action: 'installed' })),
  logs: async () => logs.parse(await command({ action: 'logs' })),
  exportLogs: async (entries: LogEntry[]) =>
    z
      .object({ path: z.string().min(1), count: z.number().int().positive() })
      .parse(await command({ action: 'export_logs', entries })),
  diagnostics: async (server: string | null) =>
    diagnostics.parse(await command({ action: 'diagnostics', server })),
  updateInstalled: async (item: InstalledItem) => {
    z.null().parse(
      await command({
        action: 'update_installed',
        slug: item.slug,
        kind: item.kind,
        clients: item.clients,
      }),
    );
  },
  uninstallInstalled: async (item: InstalledItem) => {
    z.null().parse(
      await command({
        action: 'uninstall_installed',
        slug: item.slug,
        kind: item.kind,
        clients: item.clients,
      }),
    );
  },
  clients: async () => clients.parse(await command({ action: 'clients' })),
  status: async (): Promise<Status | null> =>
    status.parse(await command({ action: 'status' })),
  login: async (server: string, token: string) =>
    account.parse(await command({ action: 'login', server, token })),
  logout: async () => {
    z.null().parse(await command({ action: 'logout' }));
  },
  apply: async (selected: ClientKind[]) =>
    clients.parse(await command({ action: 'apply', clients: selected })),
  remove: async (selected: ClientKind[]) =>
    clients.parse(await command({ action: 'remove', clients: selected })),
  doctor: async (server: string | null) =>
    doctor.parse(await command({ action: 'doctor', server })),
};
