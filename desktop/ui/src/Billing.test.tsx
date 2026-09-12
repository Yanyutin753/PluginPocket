import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { invoke, isTauri } from '@tauri-apps/api/core';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { App } from './App';

vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn(),
  isTauri: vi.fn(() => true),
}));
const usage = {
  items: [
    {
      id: 1,
      tool: 'search',
      cost: 1234,
      status: 'ok',
      duration_ms: 250,
      created_at: '2026-09-12T08:00:00+08:00',
      billing_role: 'default',
      multiplier_bp: 10000,
    },
  ],
  next_cursor: '',
};
const ledger = {
  items: [
    {
      id: 2,
      delta: 2000,
      kind: 'redemption',
      note: 'Coupon',
      created_at: '2026-09-12T08:00:00Z',
      balance_after: 3000,
    },
    {
      id: 1,
      delta: -1234,
      kind: 'reservation',
      note: 'search',
      created_at: '2026-09-12T07:00:00Z',
      balance_after: 1000,
    },
  ],
  next_cursor: '',
};
let readUsage: () => Promise<unknown>;
let readLedger: () => Promise<unknown>;
beforeEach(() => {
  vi.mocked(isTauri).mockReturnValue(true);
  readUsage = async () => usage;
  readLedger = async () => ledger;
  vi.mocked(invoke)
    .mockReset()
    .mockImplementation(async (_name, args) => {
      switch ((args as { command: { action: string } }).command.action) {
        case 'status':
          return {
            account: { username: 'alice', balance: 42, tools: ['search'] },
            clients: [],
          };
        case 'clients':
          return [];
        case 'logs':
          return [
            {
              timestamp: 1789189200000,
              level: 'info',
              action: 'apply',
              message: '客户端配置已写入',
            },
          ];
        case 'usage':
          return readUsage();
        case 'ledger':
          return readLedger();
        default:
          throw new Error('unexpected action');
      }
    });
});
function mount() {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <App />
    </QueryClientProvider>,
  );
}
async function openLogs() {
  const user = userEvent.setup();
  mount();
  await user.click(screen.getByRole('button', { name: '运行日志' }));
  await screen.findByText('客户端配置已写入');
  return user;
}
it('switches billing tabs by keyboard and reads formatted real usage and signed ledger values', async () => {
  const user = await openLogs();
  const local = screen.getByRole('tab', { name: '本地操作' });
  expect(local).toHaveAttribute('aria-selected', 'true');
  local.focus();
  await user.keyboard('{ArrowRight}{Enter}');
  const panel = await screen.findByRole('tabpanel', { name: '用量明细' });
  expect(await within(panel).findByText('1,234 credits')).toBeInTheDocument();
  expect(within(panel).getByText('成功')).toBeInTheDocument();
  expect(within(panel).getByText('250 ms')).toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: '导出日志' }),
  ).not.toBeInTheDocument();
  screen.getByRole('tab', { name: '用量明细' }).focus();
  await user.keyboard('{End} ');
  const entries = await screen.findByRole('tabpanel', { name: '账变记录' });
  expect(await within(entries).findByText('+2,000 credits')).toHaveClass(
    'amount-positive',
  );
  expect(within(entries).getByText('−1,234 credits')).toHaveClass(
    'amount-negative',
  );
  expect(within(entries).getByText('兑换入账')).toBeInTheDocument();
  expect(within(entries).getByText('3,000 credits')).toBeInTheDocument();
  await user.keyboard('{Home}{Enter}');
  expect(screen.getByText('客户端配置已写入')).toBeInTheDocument();
});
it.each(['用量明细', '账变记录'])(
  'announces loading and safe failure, then retries %s with the keyboard',
  async (tab) => {
    let reject: (reason: unknown) => void = () => {};
    const pending = () =>
      new Promise((_resolve, fail) => {
        reject = fail;
      });
    if (tab === '用量明细') readUsage = pending;
    else readLedger = pending;
    const user = await openLogs();
    await user.click(screen.getByRole('tab', { name: tab }));
    expect(
      screen.getByRole('status', { name: '正在读取账单' }),
    ).toBeInTheDocument();
    reject(new Error('secret ppt_private https://user:password@server'));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '账单暂不可用，请在概览登录或重试读取。',
    );
    expect(
      screen.queryByText(/ppt_private|password@server/),
    ).not.toBeInTheDocument();
    readUsage = async () => usage;
    readLedger = async () => ledger;
    screen.getByRole('button', { name: '重试读取账单' }).focus();
    await user.keyboard('{Enter}');
    await screen.findByRole('table');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  },
);
it.each(['用量明细', '账变记录'])(
  'rejects malformed %s responses and keeps empty results honest',
  async (tab) => {
    const malformed = async () => ({
      items: [{ tool: 'bad data', cost: 'fake' }],
      next_cursor: '',
    });
    readUsage = malformed;
    readLedger = malformed;
    const user = await openLogs();
    await user.click(screen.getByRole('tab', { name: tab }));
    await screen.findByRole('alert');
    expect(screen.queryByText('bad data')).not.toBeInTheDocument();
    readUsage = async () => ({ items: [], next_cursor: '' });
    readLedger = readUsage;
    await user.click(screen.getByRole('button', { name: '重试读取账单' }));
    await screen.findByText('暂无记录');
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  },
);
it('keeps billing unavailable in browser preview without invoking native commands', async () => {
  vi.mocked(isTauri).mockReturnValue(false);
  const user = userEvent.setup();
  mount();
  await user.click(screen.getByRole('button', { name: '运行日志' }));
  for (const tab of ['用量明细', '账变记录']) {
    await user.click(screen.getByRole('tab', { name: tab }));
    expect(
      screen.getByText('请在桌面应用中登录后查看真实账单。'),
    ).toBeInTheDocument();
  }
  expect(invoke).not.toHaveBeenCalled();
});
