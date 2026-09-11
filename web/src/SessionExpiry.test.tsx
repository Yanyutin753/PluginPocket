import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import App from './App';
import { ApiError } from './features/account/api';

it('clears private data when a 401 arrived before the session listener mounted', async () => {
  localStorage.clear();
  window.history.replaceState({}, '', '/tokens');
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  client.setQueryData(['private-ledger'], { balance: 99 });
  await expect(
    client.fetchQuery({
      queryKey: ['expired-session-request'],
      queryFn: async () => {
        throw new ApiError(401, 'unauthorized');
      },
    }),
  ).rejects.toThrow();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      Promise.resolve(
        url.endsWith('/account/me') || url.endsWith('/auth/refresh')
          ? Response.json({ error: 'unauthorized' }, { status: 401 })
          : Response.json({ items: [], next_cursor: '' }),
      ),
    ),
  );
  render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
  expect(
    await screen.findByRole('heading', { name: '登录 PluginPocket' }),
  ).toBeVisible();
  expect(client.getQueryData(['private-ledger'])).toBeUndefined();
  expect(
    screen.queryByRole('button', { name: '创建令牌' }),
  ).not.toBeInTheDocument();
});
