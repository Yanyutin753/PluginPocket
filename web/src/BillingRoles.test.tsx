import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { beforeAll, expect, it, vi } from 'vitest';
import AdminPage from './features/account/AdminPage';
import UsagePage from './features/account/UsagePage';
import PlansPage from './features/operations/PlansPage';
import { PreferencesProvider } from './i18n';

beforeAll(() => {
  Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
    configurable: true,
    value: vi.fn(),
  });
});

const roles = [
  { name: 'default', multiplier_bp: 10000, description: 'Standard rate' },
  { name: 'member', multiplier_bp: 8000, description: 'Member rate' },
  { name: 'vip', multiplier_bp: 6000, description: 'VIP rate' },
];

function mount(ui: React.ReactElement) {
  localStorage.clear();
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <PreferencesProvider>
        <MemoryRouter>{ui}</MemoryRouter>
      </PreferencesProvider>
    </QueryClientProvider>,
  );
}

it('shows the billing role and multiplier on usage rows', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Response.json({
        items: [
          {
            id: 7,
            tool: 'echo',
            cost: 3,
            status: 'ok',
            duration_ms: 15,
            created_at: '2026-09-11T08:00:00Z',
            billing_role: 'vip',
            multiplier_bp: 6000,
          },
        ],
        next_cursor: '',
      }),
    ),
  );
  mount(<UsagePage />);
  expect(await screen.findByText('vip ×0.6')).toBeTruthy();
});

it('lets admins assign a billing role from the users table', async () => {
  const calls: Array<{ url: string; body?: string }> = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((raw: string, init: RequestInit = {}) => {
      calls.push({ url: raw, body: init.body ? String(init.body) : undefined });
      if (raw.includes('/account/me')) {
        return Response.json({
          user: {
            id: 1,
            username: 'admin',
            role: 'admin',
            billing_role: 'default',
            balance: 0,
            enabled: true,
          },
          summary: { today_calls: 0, month_cost: 0, token_count: 0 },
        });
      }
      if (raw.includes('/admin/billing-roles')) {
        return Response.json({ items: roles });
      }
      if (raw.includes('/admin/users')) {
        return Response.json({
          items: [
            {
              id: 2,
              username: 'alice',
              role: 'user',
              billing_role: 'vip',
              balance: 10,
              enabled: true,
            },
          ],
          next_cursor: '',
        });
      }
      return Response.json({});
    }),
  );
  mount(<AdminPage />);
  const combo = await screen.findByRole('combobox', {
    name: '设置 alice 的计费角色',
  });
  expect(combo).toHaveTextContent('vip ×0.6');
  const user = userEvent.setup();
  await user.click(combo);
  await screen.findByRole('listbox');
  await user.click(screen.getByRole('option', { name: 'member ×0.8' }));
  await waitFor(() => {
    const patch = calls.find(
      (call) =>
        call.url.includes('/admin/users/2') && call.body?.includes('member'),
    );
    expect(patch).toBeTruthy();
    expect(JSON.parse(patch?.body ?? '{}')).toEqual({ billing_role: 'member' });
  });
});

it('edits billing role multipliers on the plans page', async () => {
  const calls: Array<{ url: string; body?: string }> = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((raw: string, init: RequestInit = {}) => {
      calls.push({ url: raw, body: init.body ? String(init.body) : undefined });
      if (raw.includes('/admin/billing-roles')) {
        if (init.method === 'PATCH') return Response.json({ item: roles[2] });
        return Response.json({ items: roles });
      }
      if (raw.includes('/admin/plans')) {
        return Response.json({ items: [], next_cursor: '' });
      }
      return Response.json({});
    }),
  );
  mount(<PlansPage />);
  const multiplier = await screen.findByLabelText('vip 倍率（×）');
  expect(multiplier).toHaveValue(0.6);
  await userEvent.clear(multiplier);
  await userEvent.type(multiplier, '0.7');
  await userEvent.click(screen.getByRole('button', { name: '保存 vip' }));
  await waitFor(() => {
    const patch = calls.find(
      (call) => call.url.includes('/admin/billing-roles/vip') && call.body,
    );
    expect(patch).toBeTruthy();
    const body = JSON.parse(patch?.body ?? '{}');
    expect(body.multiplier_bp).toBe(7000);
    expect(body.description).toBe('VIP rate');
  });
});
