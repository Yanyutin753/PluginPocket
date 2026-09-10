import {
  onlineManager,
  QueryClient,
  QueryClientProvider,
} from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import App from './App';

function renderApp() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
}

const health = { status: 'ok', service: 'loadout', version: 'test-version' };

describe('gateway connection', () => {
  it('ends a hanging request with a recoverable timeout', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        (_url: string, init: RequestInit) =>
          new Promise<Response>((_resolve, reject) => {
            init.signal?.addEventListener(
              'abort',
              () => reject(init.signal?.reason),
              { once: true },
            );
          }),
      ),
    );
    renderApp();
    expect(screen.getByRole('button', { name: '检查连接' })).toBeDisabled();
    await waitFor(
      () =>
        expect(screen.getByRole('status')).toHaveTextContent('暂时无法连接'),
      { timeout: 6_500 },
    );
    expect(screen.getByRole('button', { name: '检查连接' })).toBeEnabled();
  }, 8_000);

  it('reports a failed manual check even when the browser reports offline', async () => {
    const fetch = vi
      .fn()
      .mockResolvedValueOnce(Response.json(health))
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce(Response.json(health));
    vi.stubGlobal('fetch', fetch);
    renderApp();
    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('服务已连接'),
    );
    onlineManager.setOnline(false);
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: '检查连接' }));
    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('暂时无法连接'),
    );
    expect(screen.queryByText('test-version')).not.toBeInTheDocument();
    onlineManager.setOnline(true);
    await user.click(screen.getByRole('button', { name: '检查连接' }));
    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('服务已连接'),
    );
    expect(fetch).toHaveBeenCalledTimes(3);
  });

  it('allows keyboard access to the check button and scrollable command', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json(health)));
    renderApp();
    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('服务已连接'),
    );
    const user = userEvent.setup();
    await user.tab();
    expect(screen.getByRole('button', { name: '检查连接' })).toHaveFocus();
    await user.tab();
    expect(screen.getByLabelText('CLI 检查命令')).toHaveFocus();
  });

  it('shows pending state until a valid health response arrives', async () => {
    let resolve!: (value: Response) => void;
    const fetch = vi.fn(
      () =>
        new Promise<Response>((done) => {
          resolve = done;
        }),
    );
    vi.stubGlobal('fetch', fetch);
    renderApp();
    expect(screen.getByRole('status')).toHaveTextContent('正在检查');
    expect(screen.getByRole('button', { name: '检查连接' })).toBeDisabled();
    resolve(Response.json(health));
    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('服务已连接'),
    );
    expect(screen.getByText('test-version')).toBeVisible();
    expect(fetch).toHaveBeenCalledWith(
      '/api/v1/health',
      expect.objectContaining({
        signal: expect.any(AbortSignal),
        cache: 'no-store',
      }),
    );
  });

  it('shows a recoverable error and allows retry', async () => {
    const fetch = vi
      .fn()
      .mockRejectedValueOnce(new Error('private-network-details'))
      .mockResolvedValueOnce(Response.json(health));
    vi.stubGlobal('fetch', fetch);
    renderApp();
    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('暂时无法连接'),
    );
    expect(
      screen.queryByText('private-network-details'),
    ).not.toBeInTheDocument();
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: '检查连接' }));
    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('服务已连接'),
    );
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it.each([
    Response.json(health, { status: 503 }),
    Response.json({ ...health, service: 'other' }),
    Response.json({ ...health, status: 'down' }),
    Response.json({ status: 'ok', service: 'loadout' }),
    Response.json({ ...health, version: 'test\u001b[2J\nfake' }),
    new Response('<html>not the API</html>'),
  ])(
    'does not mistake an error or unrelated server for a healthy gateway',
    async (response) => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response));
      renderApp();
      await waitFor(() =>
        expect(screen.getByRole('status')).toHaveTextContent('暂时无法连接'),
      );
    },
  );

  it('cancels the request when the page unmounts', () => {
    const fetch = vi.fn(() => new Promise<Response>(() => {}));
    vi.stubGlobal('fetch', fetch);
    const { unmount } = renderApp();
    const signal = (fetch.mock.calls[0] as unknown as [string, RequestInit])[1]
      .signal;
    unmount();
    expect(signal?.aborted).toBe(true);
  });
});
