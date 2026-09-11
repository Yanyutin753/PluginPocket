import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import App from './App';

const account = {
  user: { id: 1, username: 'alice', role: 'user', balance: 0, enabled: true },
  summary: { today_calls: 0, month_cost: 0, token_count: 0 },
};
function mount(path: string) {
  window.history.replaceState({}, '', path);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
}
it('replaces a cached account when the public page observes a different cookie identity', async () => {
  loggedIn();
  window.history.replaceState({}, '', '/');
  const client = new QueryClient();
  client.setQueryData(['account'], {
    ...account,
    user: { ...account.user, id: 2, username: 'previous' },
  });
  render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
  const entry = await waitFor(() =>
    within(screen.getByRole('navigation', { name: '主导航' })).getByRole(
      'link',
      { name: '进入工作空间' },
    ),
  );
  await userEvent.setup().click(entry);
  expect(
    await within(
      await screen.findByRole('button', { name: '账号菜单' }),
    ).findByText('alice'),
  ).toBeVisible();
});
it('keeps a login session-check outage retryable without exposing the password form', async () => {
  let down = true;
  vi.stubGlobal(
    'fetch',
    vi.fn(async () =>
      down
        ? Response.json({ error: 'internal_error' }, { status: 503 })
        : Response.json({ error: 'unauthorized' }, { status: 401 }),
    ),
  );
  mount('/login');
  await screen.findByRole('alert');
  expect(screen.queryByLabelText('密码')).not.toBeInTheDocument();
  down = false;
  await userEvent.setup().click(screen.getByRole('button', { name: '重试' }));
  await waitFor(() => expect(screen.getByLabelText('密码')).toBeVisible());
});
function loggedIn() {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      Promise.resolve(
        Response.json(
          url.endsWith('/account/me')
            ? account
            : { items: [], origin: '', next_cursor: '' },
        ),
      ),
    ),
  );
}
it('restores the existing session on a direct login visit without submitting credentials', async () => {
  loggedIn();
  mount('/login');
  expect(await screen.findByRole('button', { name: '账号菜单' })).toBeVisible();
  expect(window.location.pathname).toBe('/overview');
  expect(screen.queryByLabelText('密码')).not.toBeInTheDocument();
});
it('shows a keyboard accessible workspace entry on the public plugin page for an authenticated visitor', async () => {
  loggedIn();
  mount('/plugins?page=3');
  await screen.findByRole('button', { name: '账号菜单' });
  const nav = await screen.findByRole('navigation', { name: '主导航' });
  const entry = await within(nav).findByRole('link', { name: '概览' });
  expect(
    within(nav).queryByRole('link', { name: '登录' }),
  ).not.toBeInTheDocument();
  entry.focus();
  await userEvent.setup().keyboard('{Enter}');
  expect(await screen.findByRole('button', { name: '账号菜单' })).toBeVisible();
});
