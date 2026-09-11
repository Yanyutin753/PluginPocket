import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { StrictMode } from 'react';
import { expect, it, vi } from 'vitest';
import App from './App';

it('clears another account data and one-time secrets when shared cookie identity changes', async () => {
  let current = 'alice';
  const token = {
    id: 1,
    name: 'Alice private token',
    prefix: 'ldt_alice',
    wallet_id: 1,
    created_at: '2026-09-10T10:00:00Z',
    revoked_at: null,
    last_used_at: null,
  };
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init: RequestInit = {}) => {
      if (url === '/api/v1/account/me')
        return Promise.resolve(
          Response.json({
            user: {
              id: current === 'alice' ? 1 : 2,
              username: current,
              role: 'user',
              balance: 1,
              enabled: true,
            },
            summary: { today_calls: 0, month_cost: 0, token_count: 0 },
          }),
        );
      if (url === '/api/v1/account/tokens' && init.method === 'POST')
        return Promise.resolve(
          Response.json({ token: 'ldt_alice_secret', item: token }),
        );
      if (url.startsWith('/api/v1/account/tokens'))
        return Promise.resolve(
          Response.json({
            items: current === 'alice' ? [token] : [],
            next_cursor: '',
          }),
        );
      return Promise.resolve(Response.json({ items: [], next_cursor: '' }));
    }),
  );
  window.history.replaceState({}, '', '/tokens');
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <StrictMode>
        <App />
      </StrictMode>
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  await user.click(await screen.findByRole('button', { name: '创建令牌' }));
  await user.type(await screen.findByLabelText('令牌名称'), 'Alice device');
  await user.click(screen.getByRole('button', { name: '创建令牌' }));
  await screen.findByText('ldt_alice_secret');
  current = 'bob';
  await act(async () => {
    await client.invalidateQueries({ queryKey: ['account'] });
  });
  await within(screen.getByRole('button', { name: '账号菜单' })).findByText(
    'bob',
  );
  expect(screen.queryByText('ldt_alice_secret')).not.toBeInTheDocument();
  expect(screen.queryByText('Alice private token')).not.toBeInTheDocument();
  client.clear();
});

it('clears team invitation when route changes directly between team identities', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/v1/account/me')
        return Promise.resolve(
          Response.json({
            user: {
              id: 1,
              username: 'alice',
              role: 'user',
              balance: 1,
              enabled: true,
            },
            summary: { today_calls: 0, month_cost: 0, token_count: 0 },
          }),
        );
      if (url === '/api/v1/account/teams/1/invites')
        return Promise.resolve(
          Response.json({
            code: 'invite_for_team_one',
            expires_at: '2026-09-11T10:00:00Z',
          }),
        );
      if (/^\/api\/v1\/account\/teams\/[12]$/.test(url)) {
        const id = Number(url.slice(-1));
        return Promise.resolve(
          Response.json({
            item: {
              id,
              name: `Team ${id}`,
              role: 'owner',
              balance: 1,
              seat_limit: 5,
            },
          }),
        );
      }
      return Promise.resolve(Response.json({ items: [], next_cursor: '' }));
    }),
  );
  window.history.replaceState({}, '', '/teams/1');
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <StrictMode>
        <App />
      </StrictMode>
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  await user.click(await screen.findByRole('button', { name: '邀请成员' }));
  await user.click(await screen.findByRole('button', { name: '生成邀请' }));
  await screen.findByText('invite_for_team_one');
  await act(async () => {
    window.history.pushState({}, '', '/teams/2');
    window.dispatchEvent(new PopStateEvent('popstate'));
  });
  await screen.findByRole('heading', { name: 'Team 2' });
  expect(screen.queryByText('invite_for_team_one')).not.toBeInTheDocument();
  client.clear();
});

it('a previous account mutation cannot repopulate the new account cache', async () => {
  let current = 'alice';
  let finish!: (response: Response) => void;
  const pending = new Promise<Response>((resolve) => {
    finish = resolve;
  });
  const team = {
    id: 1,
    name: 'Shared team',
    role: 'owner',
    balance: 1,
    seat_limit: 5,
  };
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init: RequestInit = {}) => {
      if (url === '/api/v1/account/me')
        return Promise.resolve(
          Response.json({
            user: {
              id: current === 'alice' ? 1 : 2,
              username: current,
              role: 'user',
              balance: 1,
              enabled: true,
            },
            summary: { today_calls: 0, month_cost: 0, token_count: 0 },
          }),
        );
      if (url === '/api/v1/account/teams/1/fund' && init.method === 'POST')
        return pending;
      if (url === '/api/v1/account/teams/1')
        return Promise.resolve(Response.json({ item: team }));
      return Promise.resolve(Response.json({ items: [], next_cursor: '' }));
    }),
  );
  window.history.replaceState({}, '', '/teams/1');
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <StrictMode>
        <App />
      </StrictMode>
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  await user.click(await screen.findByRole('button', { name: '转入团队额度' }));
  await user.type(await screen.findByLabelText('转入额度'), '5');
  await user.click(screen.getByRole('button', { name: '确认转入' }));
  current = 'bob';
  await act(async () => {
    await client.invalidateQueries({ queryKey: ['account'] });
  });
  await within(screen.getByRole('button', { name: '账号菜单' })).findByText(
    'bob',
  );
  await act(async () => {
    finish(
      Response.json({ item: { ...team, name: 'Alice old private result' } }),
    );
    await pending;
  });
  expect(screen.getByRole('heading', { name: 'Shared team' })).toBeVisible();
  expect(
    screen.queryByText('Alice old private result'),
  ).not.toBeInTheDocument();
});
