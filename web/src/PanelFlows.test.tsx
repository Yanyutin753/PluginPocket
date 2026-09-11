import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { expect, it, vi } from 'vitest';
import TokensPage from './features/account/TokensPage';
import BillingPage from './features/operations/BillingPage';
import CodesPage from './features/operations/CodesPage';
import PlansPage from './features/operations/PlansPage';
import SettingsPage from './features/operations/SettingsPage';
import SystemSettingsPage from './features/operations/SystemSettingsPage';
import TeamsPage from './features/operations/TeamsPage';
import ToolsPage from './features/operations/ToolsPage';

function mount(page: React.ReactNode) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      Promise.resolve(
        Response.json(
          url.includes('/admin/settings')
            ? {
                item: {
                  revision: 1,
                  initial_credits: 100,
                  github_enabled: false,
                  github_client_id: '',
                  github_org: '',
                  github_client_secret_set: false,
                  smtp_enabled: false,
                  smtp_address: '',
                  smtp_from: '',
                  smtp_username: '',
                  smtp_password_set: false,
                },
                secret_writes_available: true,
              }
            : url.includes('/account/email')
              ? { email: null, verified_at: null, configured: true }
              : {
                  items: [
                    {
                      id: 1,
                      key: 'echo',
                      name: 'Echo',
                      description: 'Return a message',
                      kind: 'builtin',
                      enabled: true,
                      units_per_call: 1,
                      input_schema: { type: 'object' },
                      configured: true,
                    },
                  ],
                  next_cursor: '',
                },
        ),
      ),
    ),
  );
  render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: {
            queries: { retry: false },
            mutations: { retry: false },
          },
        })
      }
    >
      <MemoryRouter>{page}</MemoryRouter>
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

it('opens tool parameters in a named drawer and returns keyboard focus', async () => {
  const user = mount(<ToolsPage />);
  const trigger = await screen.findByRole('button', { name: '查看参数 Echo' });
  await user.click(trigger);
  const drawer = screen.getByRole('dialog', { name: 'Echo' });
  expect(within(drawer).getByText(/"type": "object"/)).toBeVisible();
  await user.keyboard('{Escape}');
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(trigger).toHaveFocus();
});

it.each([
  ['添加套餐', '套餐名称', PlansPage],
  ['新建团队', '团队名称', TeamsPage],
  ['生成兑换码', '兑换额度', CodesPage],
  ['邮箱验证', '邮箱地址', SettingsPage],
  ['创建令牌', '令牌名称', TokensPage],
  ['编辑系统配置', '注册赠送额度', SystemSettingsPage],
  ['兑换额度', '兑换码', BillingPage],
] as const)('keeps %s fields behind a drawer', async (title, field, Page) => {
  const user = mount(<Page />);
  const trigger = await screen.findByRole('button', { name: title });
  expect(screen.queryByLabelText(field)).not.toBeInTheDocument();
  await user.click(trigger);
  const drawer = screen.getByRole('dialog', { name: title });
  expect(within(drawer).getByLabelText(field)).toBeVisible();
  await user.keyboard('{Escape}');
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(trigger).toHaveFocus();
});

it('keeps marketplace browsing off the tool management list', async () => {
  const user = mount(<ToolsPage admin />);
  expect(
    screen.queryByRole('region', { name: '插件市场' }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '插件市场' }));
  expect(screen.getByRole('dialog', { name: '插件市场' })).toBeVisible();
});
