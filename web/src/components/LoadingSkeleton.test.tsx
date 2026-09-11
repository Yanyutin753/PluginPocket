import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import SettingsPage from '@/features/operations/SettingsPage';
import { LoadingSkeleton } from './LoadingSkeleton';

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

function pendingRequest() {
  let resolve: (response: Response) => void = () => {};
  const promise = new Promise<Response>((done) => {
    resolve = done;
  });
  vi.mocked(fetch).mockReturnValueOnce(promise);
  return resolve;
}

function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <SettingsPage />
    </QueryClientProvider>,
  );
  return client;
}

const email = {
  email: 'alice@example.com',
  verified_at: null,
  configured: true,
};

it('explains page preparation with a live connection status and no fictional progress', () => {
  render(<LoadingSkeleton variant="page" />);
  expect(
    screen.getByRole('heading', { name: '正在准备你的工作空间' }),
  ).toBeVisible();
  expect(screen.getByRole('status')).toHaveTextContent('正在连接，请稍候…');
  expect(
    screen.getByRole('region', { name: '正在加载，请稍候…' }),
  ).toHaveAttribute('aria-busy', 'true');
  expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
  expect(screen.queryByRole('img')).not.toBeInTheDocument();
});

it('marks pending content busy and replaces the placeholder with the real form when its request settles', async () => {
  const resolve = pendingRequest();
  mount();
  const placeholder = screen.getByRole('region', { name: '正在加载，请稍候…' });
  expect(placeholder).toHaveAttribute('aria-busy', 'true');
  expect(screen.getByRole('status')).toHaveTextContent('正在加载，请稍候…');
  expect(
    screen.queryByRole('textbox', { name: '邮箱地址' }),
  ).not.toBeInTheDocument();
  resolve(Response.json(email));
  await userEvent
    .setup()
    .click(await screen.findByRole('button', { name: '邮箱验证' }));
  expect(await screen.findByRole('textbox', { name: '邮箱地址' })).toHaveValue(
    email.email,
  );
  expect(
    screen.queryByRole('region', { name: '正在加载，请稍候…' }),
  ).not.toBeInTheDocument();
});

it('removes the busy placeholder on failure and provides a working retry', async () => {
  const user = userEvent.setup();
  const resolve = pendingRequest();
  mount();
  expect(
    screen.getByRole('region', { name: '正在加载，请稍候…' }),
  ).toHaveAttribute('aria-busy', 'true');
  resolve(Response.json({}, { status: 503 }));
  expect(await screen.findByRole('alert')).toBeVisible();
  expect(
    screen.queryByRole('region', { name: '正在加载，请稍候…' }),
  ).not.toBeInTheDocument();
  vi.mocked(fetch).mockResolvedValueOnce(Response.json(email));
  await user.click(screen.getByRole('button', { name: '重试' }));
  await userEvent
    .setup()
    .click(await screen.findByRole('button', { name: '邮箱验证' }));
  expect(await screen.findByRole('textbox', { name: '邮箱地址' })).toHaveValue(
    email.email,
  );
});

it('keeps existing content available during background refresh', async () => {
  vi.mocked(fetch).mockResolvedValueOnce(Response.json(email));
  const client = mount();
  await userEvent
    .setup()
    .click(await screen.findByRole('button', { name: '邮箱验证' }));
  await screen.findByRole('textbox', { name: '邮箱地址' });
  const resolve = pendingRequest();
  void client.invalidateQueries({ queryKey: ['/account/email'] });
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  expect(screen.getByRole('textbox', { name: '邮箱地址' })).toHaveValue(
    email.email,
  );
  expect(
    screen.queryByRole('region', { name: '正在加载，请稍候…' }),
  ).not.toBeInTheDocument();
  resolve(Response.json(email));
});
