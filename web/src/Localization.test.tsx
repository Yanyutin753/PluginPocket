import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import App from './App';
import { chooseOption } from './test/select';

beforeEach(() => localStorage.setItem('pluginpocket.locale', 'en'));
afterEach(() => localStorage.clear());

function mount(
  path: string,
  fail = false,
  handler?: (url: string, init: RequestInit) => Response | undefined,
) {
  window.history.replaceState({}, '', path);
  vi.stubGlobal('fetch', (url: string, init: RequestInit = {}) => {
    const response = handler?.(url, init);
    if (response) return Promise.resolve(response);
    if (url === '/api/v1/account/me')
      return Promise.resolve(
        Response.json({
          user: {
            id: 1,
            username: '中文用户名',
            role: 'admin',
            balance: 1234,
            enabled: true,
          },
          summary: { today_calls: 0, month_cost: 0, token_count: 0 },
        }),
      );
    if (url === '/api/v1/account/email')
      return Promise.resolve(
        Response.json({ email: null, verified_at: null, configured: false }),
      );
    if (url === '/api/v1/meta')
      return Promise.resolve(
        Response.json({ github: false, email: false, payments: false }),
      );
    if (fail)
      return Promise.resolve(
        Response.json(
          { error: 'internal_error', secret: 'private-server-detail' },
          { status: 500 },
        ),
      );
    return Promise.resolve(Response.json({ items: [], next_cursor: '' }));
  });
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
  return () => {
    fail = false;
  };
}

it.each([
  ['/tokens', 'Gateway tokens', 'Create token'],
  ['/tools', 'Tool catalog', 'No tools available'],
  ['/billing', 'Billing', 'Redeem code'],
  ['/settings', 'Account settings', 'Email verification'],
  ['/admin/tools', 'Tool management', 'Add tool'],
  ['/admin/plans', 'Plan management', 'Create plan'],
  ['/admin/codes', 'Redemption codes', 'Create redemption code'],
  ['/teams', 'My teams', 'Accept invitation'],
  ['/usage', 'Usage', 'No tool calls yet'],
  ['/admin/usage', 'Global usage', 'Export CSV'],
  ['/admin/ledger', 'Credit audit', 'Filter transactions'],
  ['/device', 'Device access', 'Approve this device'],
])('renders English content on %s', async (path, heading, label) => {
  mount(path);
  expect(await screen.findByRole('heading', { name: heading })).toBeVisible();
  expect((await screen.findAllByText(label, { exact: true }))[0]).toBeVisible();
  expect(screen.getAllByText('中文用户名').length).toBeGreaterThan(0);
});

it('translates safe API errors and allows keyboard retry in English', async () => {
  const recover = mount('/tools', true);
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'The service hit an internal error',
  );
  expect(screen.queryByText('private-server-detail')).not.toBeInTheDocument();
  recover();
  const retry = screen.getByRole('button', { name: 'Retry' });
  retry.focus();
  await userEvent.setup().keyboard('{Enter}');
  expect(await screen.findByText('No tools available')).toBeVisible();
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});

it('falls back to the generic message for unknown or non-JSON errors', async () => {
  mount('/tools', false, (url) =>
    url.startsWith('/api/v1/tools')
      ? new Response('upstream exploded', { status: 502 })
      : undefined,
  );
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Unable to complete the request',
  );
  expect(screen.queryByText('upstream exploded')).not.toBeInTheDocument();
});

it('keeps user-provided names intact and updates token dates when switching language', async () => {
  mount('/tokens', false, (url) =>
    url.startsWith('/api/v1/account/tokens')
      ? Response.json({
          items: [
            {
              id: 1,
              name: '个人设置',
              prefix: 'ppt_demo',
              wallet_id: 1,
              created_at: '2026-09-10T10:00:00Z',
              revoked_at: null,
              last_used_at: null,
            },
          ],
          next_cursor: '',
        })
      : undefined,
  );
  expect(
    await screen.findByRole('heading', { name: '个人设置' }),
  ).toBeVisible();
  const formatted = new Intl.DateTimeFormat('en', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date('2026-09-10T10:00:00Z'));
  expect(
    screen.getByText(`Created ${formatted} · Last used: Never used`),
  ).toBeVisible();
  await chooseOption(
    userEvent.setup(),
    screen.getByRole('combobox', { name: 'Language' }),
    '中文',
  );
  expect(screen.getByRole('heading', { name: '个人设置' })).toBeVisible();
  expect(screen.getByRole('button', { name: '撤销 个人设置' })).toBeVisible();
  expect(screen.getByText(/最近使用：从未使用/)).toBeVisible();
});

it('shows English form validation and preserves tool configuration for recovery', async () => {
  mount('/admin/tools');
  const user = userEvent.setup();
  await user.click(await screen.findByRole('button', { name: 'Add tool' }));
  await user.type(screen.getByLabelText('Tool identifier'), 'remote');
  await user.type(screen.getByLabelText('Display name'), '用户工具');
  await chooseOption(
    user,
    screen.getByLabelText('Connection type'),
    'HTTP service',
  );
  await chooseOption(
    user,
    screen.getByLabelText('Input method'),
    'Advanced JSON',
  );
  await user.type(
    screen.getByLabelText('Connection configuration (JSON)'),
    'not-json',
  );
  await user.click(screen.getByRole('button', { name: 'Save tool' }));
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Connection settings must be a JSON object',
  );
  expect(screen.getByLabelText('Connection configuration (JSON)')).toHaveValue(
    'not-json',
  );
  expect(screen.getByLabelText('Display name')).toHaveValue('用户工具');
});

it.each([
  ['/health', 'Connection status', '接入状态'],
  ['/verify-email', 'Verify email', '验证邮箱'],
])(
  'offers language and theme controls on public page %s',
  async (path, englishHeading, chineseHeading) => {
    mount(path);
    expect(
      await screen.findByRole('heading', { name: englishHeading }),
    ).toBeVisible();
    const user = userEvent.setup();
    await chooseOption(
      user,
      screen.getByRole('combobox', { name: 'Language' }),
      '中文',
    );
    expect(screen.getByRole('heading', { name: chineseHeading })).toBeVisible();
    await chooseOption(
      user,
      screen.getByRole('combobox', { name: '外观' }),
      '深色',
    );
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark');
  },
);
