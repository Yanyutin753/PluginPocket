import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import App from './App';

beforeEach(() => {
  localStorage.clear();
  window.history.replaceState({}, '', '/');
});
it('opens the public landing page without requesting a private account and exposes working signup/login destinations', async () => {
  const fetcher = vi.fn(() =>
    Promise.resolve(Response.json({ error: 'unauthorized' }, { status: 401 })),
  );
  vi.stubGlobal('fetch', fetcher);
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <App />
    </QueryClientProvider>,
  );
  expect(
    await screen.findByRole('heading', {
      level: 1,
      name: '给你的 AI，装上超能力',
    }),
  ).toBeVisible();
  expect(
    screen
      .getAllByRole('link', { name: '登录' })
      .every((link) => link.getAttribute('href') === '/login'),
  ).toBe(true);
  expect(
    screen.getByRole('link', { name: '创建账号，开始装备' }),
  ).toHaveAttribute('href', '/register');
  expect(fetcher).not.toHaveBeenCalled();
});
