import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import App from './App';

beforeEach(() => {
  localStorage.clear();
  window.history.replaceState({}, '', '/login');
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(
        Response.json({ github: false, email: false, payments: false }),
      ),
    ),
  );
});
it('lets keyboard users reveal and conceal the password without submitting or losing it', async () => {
  const user = userEvent.setup();
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <App />
    </QueryClientProvider>,
  );
  const password = await screen.findByLabelText('密码', { exact: true });
  await user.type(password, 'my-local-test-password');
  expect(password).toHaveAttribute('type', 'password');
  await user.tab();
  const toggle = screen.getByRole('button', { name: '显示密码' });
  expect(toggle).toHaveFocus();
  await user.keyboard('{Enter}');
  expect(password).toHaveAttribute('type', 'text');
  expect(password).toHaveAttribute('spellcheck', 'false');
  expect(password).toHaveAttribute('autocapitalize', 'none');
  expect(password).toHaveAttribute('autocorrect', 'off');
  expect(password).toHaveValue('my-local-test-password');
  expect(screen.getByRole('button', { name: '隐藏密码' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await user.keyboard(' ');
  expect(password).toHaveAttribute('type', 'password');
  expect(
    vi.mocked(fetch).mock.calls.every(([, init]) => init?.method !== 'POST'),
  ).toBe(true);
});
