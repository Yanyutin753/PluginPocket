import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { invoke } from '@tauri-apps/api/core';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';

vi.mock('@tauri-apps/api/core', () => ({ invoke: vi.fn() }));
const initialClients = [
  { client: 'codex', detected: true, configured: false },
  { client: 'claude', detected: true, configured: false },
  { client: 'cursor', detected: false, configured: false },
];
const account = { username: 'alice', balance: 42, tools: ['echo'] };
let loggedIn = false;
let configured = false;
beforeEach(() => {
  loggedIn = false;
  configured = false;
  vi.mocked(invoke).mockReset();
  vi.mocked(invoke).mockImplementation(async (name, args) => {
    expect(name).toBe('local_command');
    const command = (args as { command: { action: string } }).command;
    const clients = initialClients.map((client) => ({
      ...client,
      configured: configured && client.client === 'codex',
    }));
    switch (command.action) {
      case 'clients':
        return clients;
      case 'status':
        if (!loggedIn) throw new Error('not logged in');
        return { account, clients };
      case 'login':
        loggedIn = true;
        return account;
      case 'apply':
        configured = true;
        return initialClients.map((c) => ({
          ...c,
          configured: c.client === 'codex',
        }));
      case 'remove':
        configured = false;
        return initialClients;
      case 'logout':
        loggedIn = false;
        return null;
      case 'doctor':
        return { version: 'test', authenticated: loggedIn, clients };
      default:
        throw new Error('unexpected action');
    }
  });
});
function mount() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>,
  );
}
async function login(user: ReturnType<typeof userEvent.setup>) {
  await user.type(
    await screen.findByLabelText('服务地址'),
    'http://127.0.0.1:8787',
  );
  await user.type(screen.getByLabelText('Loadout 令牌'), 'ldt_private');
  await user.click(screen.getByRole('button', { name: '登录' }));
  await screen.findByText('alice');
}
describe('local desktop companion', () => {
  it('logs in, clears the secret, configures selected clients, removes and logs out through limited native commands', async () => {
    const user = userEvent.setup();
    mount();
    await login(user);
    expect(screen.queryByDisplayValue('ldt_private')).not.toBeInTheDocument();
    expect(screen.getByText('42 credits')).toBeInTheDocument();
    await user.click(screen.getByRole('checkbox', { name: /Codex/ }));
    await user.click(screen.getByRole('button', { name: '配置所选客户端' }));
    await screen.findByText('配置已写入 重启客户端后即可使用');
    expect(invoke).toHaveBeenCalledWith('local_command', {
      command: { action: 'apply', clients: ['codex'] },
    });
    await user.click(screen.getByRole('button', { name: '移除所选配置' }));
    await screen.findByText('已移除所选客户端的 Loadout 配置');
    await user.click(screen.getByRole('button', { name: '检查连接' }));
    await screen.findByText('服务可达 · 凭证有效');
    await user.click(screen.getByRole('button', { name: '退出登录' }));
    await screen.findByRole('button', { name: '登录' });
    expect(screen.queryByText('42 credits')).not.toBeInTheDocument();
  });
  it('shows safe failure feedback, clears failed secrets and allows keyboard retry', async () => {
    const user = userEvent.setup();
    mount();
    await user.type(
      await screen.findByLabelText('服务地址'),
      'https://example.com',
    );
    await user.type(screen.getByLabelText('Loadout 令牌'), 'ldt_private');
    vi.mocked(invoke).mockRejectedValueOnce(
      new Error('secret ldt_private https://user:password@server'),
    );
    await user.click(screen.getByRole('button', { name: '登录' }));
    await screen.findByText('登录失败，请检查服务地址和令牌后重试');
    expect(screen.getByLabelText('Loadout 令牌')).toHaveValue('');
    expect(screen.queryByText(/password@server/)).not.toBeInTheDocument();
    await user.click(screen.getByLabelText('Loadout 令牌'));
    await user.keyboard('ldt_new{Enter}');
    await screen.findByText('alice');
  });
  it('disables duplicate submissions while a native command is pending', async () => {
    const user = userEvent.setup();
    mount();
    await user.type(
      await screen.findByLabelText('服务地址'),
      'https://example.com',
    );
    await user.type(screen.getByLabelText('Loadout 令牌'), 'ldt_private');
    let resolve: (value: unknown) => void = () => {};
    vi.mocked(invoke).mockImplementationOnce(
      () =>
        new Promise((done) => {
          resolve = done;
        }),
    );
    await user.click(screen.getByRole('button', { name: '登录' }));
    expect(screen.getByRole('button', { name: '登录中…' })).toBeDisabled();
    resolve(account);
    await screen.findByText('alice');
  });
  it('rejects malformed native status and keeps refresh recovery available', async () => {
    loggedIn = true;
    vi.mocked(invoke).mockImplementation(async (_name, args) => {
      if (
        (args as { command: { action: string } }).command.action === 'clients'
      )
        return initialClients;
      return { account: { username: 'alice', balance: 'fake' }, clients: [] };
    });
    mount();
    await waitFor(() =>
      expect(
        screen.getByText('尚未登录或凭证不可用，请登录或刷新重试'),
      ).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: '刷新状态' })).toBeEnabled();
    expect(screen.queryByText('fake credits')).not.toBeInTheDocument();
  });
});

it('removes stale account information when refreshing revoked credentials fails', async () => {
  const user = userEvent.setup();
  mount();
  await login(user);
  vi.mocked(invoke).mockImplementation(async (_name, args) => {
    if ((args as { command: { action: string } }).command.action === 'clients')
      return initialClients;
    throw new Error('unauthorized');
  });
  await user.click(screen.getByRole('button', { name: '刷新状态' }));
  await screen.findByText('尚未登录或凭证不可用，请登录或刷新重试');
  expect(screen.queryByText('42 credits')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: '配置所选客户端' })).toBeDisabled();
});

it('offers keyboard-focusable light, dark and system appearance without native commands', async () => {
  Element.prototype.scrollIntoView = vi.fn();
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.setPointerCapture = vi.fn();
  Element.prototype.releasePointerCapture = vi.fn();
  const user = userEvent.setup();
  mount();
  await screen.findByRole('checkbox', { name: /Codex/ });
  const appearance = screen.getByRole('combobox', { name: '外观' });
  expect(appearance).toHaveTextContent('跟随系统');
  await user.tab();
  expect(appearance).toHaveFocus();
  vi.mocked(invoke).mockClear();
  await user.keyboard('{Enter}');
  await user.click(screen.getByRole('option', { name: '深色' }));
  expect(document.documentElement.style.colorScheme).toBe('dark');
  await user.click(appearance);
  await user.click(screen.getByRole('option', { name: '浅色' }));
  expect(document.documentElement.style.colorScheme).toBe('light');
  await user.click(appearance);
  await user.click(screen.getByRole('option', { name: '跟随系统' }));
  expect(document.documentElement.style.colorScheme).toBe('light dark');
  expect(invoke).not.toHaveBeenCalled();
});

it('announces initial loading placeholders independently and replaces them with real account and clients', async () => {
  let resolveStatus: (value: unknown) => void = () => {};
  let resolveClients: (value: unknown) => void = () => {};
  vi.mocked(invoke).mockImplementation(
    (_name, args) =>
      new Promise((resolve) => {
        if (
          (args as { command: { action: string } }).command.action === 'status'
        )
          resolveStatus = resolve;
        else resolveClients = resolve;
      }),
  );
  mount();
  for (const label of ['正在读取本地状态', '正在检测客户端']) {
    expect(
      screen.getByRole('status', { name: label }).closest('[aria-busy="true"]'),
    ).toBeNull();
  }
  expect(
    screen.getByRole('status', { name: '正在读取本地状态' }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole('status', { name: '正在检测客户端' }),
  ).toBeInTheDocument();
  expect(screen.getByRole('region', { name: '账号与连接' })).toHaveAttribute(
    'aria-busy',
    'true',
  );
  expect(screen.queryByLabelText('Loadout 令牌')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: '配置所选客户端' })).toBeDisabled();
  resolveStatus({ account, clients: initialClients });
  await screen.findByText('alice');
  expect(
    screen.queryByRole('status', { name: '正在读取本地状态' }),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole('status', { name: '正在检测客户端' }),
  ).toBeInTheDocument();
  resolveClients(initialClients);
  await screen.findByRole('checkbox', { name: /Codex/ });
  expect(
    screen.queryByRole('status', { name: '正在检测客户端' }),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole('region', { name: '这台电脑的客户端' }),
  ).toHaveAttribute('aria-busy', 'false');
});
