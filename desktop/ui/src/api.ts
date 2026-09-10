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
  tools: z.array(z.unknown()),
});
const status = z.object({ account, clients });
const doctor = z.object({
  version: z.string(),
  authenticated: z.boolean(),
  clients,
});
export type Status = z.infer<typeof status>;
const command = (command: Record<string, unknown>) =>
  invoke<unknown>('local_command', { command });
export const api = {
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
