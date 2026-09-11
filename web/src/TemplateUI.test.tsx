import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import App from './App';

beforeEach(() => {
  localStorage.clear();
  window.history.replaceState({}, '', '/overview');
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(
        Response.json({
          user: {
            id: 1,
            username: 'alice',
            role: 'admin',
            balance: 4200,
            enabled: true,
          },
          summary: { today_calls: 3, month_cost: 80, token_count: 1 },
        }),
      ),
    ),
  );
});
function mount() {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <App />
    </QueryClientProvider>,
  );
}
it('introduces the workshop with ordered setup instructions and a keyboard-accessible token action', async () => {
  const user = userEvent.setup();
  mount();
  expect(
    await screen.findByRole('heading', {
      name: '把 AI 的超能力，装进口袋',
      level: 1,
    }),
  ).toBeVisible();
  const steps = screen.getByRole('list', { name: '接入指南' });
  const items = within(steps).getAllByRole('listitem');
  expect(items).toHaveLength(2);
  expect(items[0]).toHaveTextContent('创建令牌');
  expect(items[1]).toHaveTextContent('连接客户端');
  const action = screen.getByRole('link', { name: '创建网关令牌' });
  action.focus();
  await user.keyboard('{Enter}');
  expect(window.location.pathname).toBe('/tokens');
});
it('groups workspace and administration links and supports keyboard navigation', async () => {
  const user = userEvent.setup();
  mount();
  const workspace = await screen.findByRole('group', { name: '工作空间' });
  const admin = screen.getByRole('group', { name: '管理后台' });
  expect(
    within(workspace).getByRole('link', { name: '网关令牌' }),
  ).toBeVisible();
  expect(
    within(workspace).queryByRole('link', { name: '用户管理' }),
  ).not.toBeInTheDocument();
  const link = within(admin).getByRole('link', { name: '用户管理' });
  link.focus();
  await user.keyboard('{Enter}');
  expect(window.location.pathname).toBe('/admin/users');
});
it('copies the complete connection commands and reports success', async () => {
  const user = userEvent.setup();
  const write = vi.spyOn(navigator.clipboard, 'writeText');
  mount();
  await user.click(await screen.findByRole('button', { name: '复制接入命令' }));
  expect(write).toHaveBeenCalledWith(
    `pluginpocket login --server ${window.location.origin}\npluginpocket apply`,
  );
  expect(await screen.findByRole('status')).toHaveTextContent('接入命令已复制');
});
it('keeps commands available for manual copying when clipboard access fails', async () => {
  const user = userEvent.setup();
  vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(
    new Error('denied'),
  );
  mount();
  await user.click(await screen.findByRole('button', { name: '复制接入命令' }));
  expect(await screen.findByRole('status')).toHaveTextContent(
    '无法自动复制，请选中命令并手动复制',
  );
  expect(screen.getByText(/pluginpocket login --server/)).toBeVisible();
});
