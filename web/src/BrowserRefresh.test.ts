import { expect, it, vi } from 'vitest';
import { fetchResponse } from './features/account/api';

const unauthorized = () =>
  Response.json({ error: 'unauthorized' }, { status: 401 });
it('shares one refresh for concurrent unauthorized requests then retries each original request', async () => {
  let refreshed = false;
  const fetcher = vi.fn(async (url: string) => {
    if (url.endsWith('/auth/refresh')) {
      refreshed = true;
      return new Response(null, { status: 204 });
    }
    return refreshed ? Response.json({ ok: true }) : unauthorized();
  });
  vi.stubGlobal('fetch', fetcher);
  const results = await Promise.all([
    fetchResponse('/account/me'),
    fetchResponse('/account/tokens'),
  ]);
  expect(results.every((r) => r.ok)).toBe(true);
  expect(
    fetcher.mock.calls.filter(([url]) => url.endsWith('/auth/refresh')),
  ).toHaveLength(1);
  expect(fetcher).toHaveBeenCalledTimes(5);
});
it('does not loop when refreshed credentials are still unauthorized', async () => {
  const fetcher = vi.fn(async (url: string) =>
    url.endsWith('/auth/refresh')
      ? new Response(null, { status: 204 })
      : unauthorized(),
  );
  vi.stubGlobal('fetch', fetcher);
  await expect(fetchResponse('/account/me')).rejects.toMatchObject({
    status: 401,
  });
  expect(fetcher).toHaveBeenCalledTimes(3);
});
it('preserves service errors on refresh instead of treating them as logout', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) =>
      url.endsWith('/auth/refresh')
        ? Response.json({ error: 'internal_error' }, { status: 503 })
        : unauthorized(),
    ),
  );
  await expect(fetchResponse('/account/me')).rejects.toMatchObject({
    status: 503,
  });
});
it('never refreshes password failures, logout, or forbidden responses', async () => {
  const fetcher = vi.fn(async () => unauthorized());
  vi.stubGlobal('fetch', fetcher);
  await expect(
    fetchResponse('/auth/login', { method: 'POST' }),
  ).rejects.toMatchObject({ status: 401 });
  await expect(
    fetchResponse('/auth/logout', { method: 'POST' }),
  ).rejects.toMatchObject({ status: 401 });
  expect(fetcher).toHaveBeenCalledTimes(2);
});
