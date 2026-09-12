import { mkdtemp, rm } from 'node:fs/promises';
import { createServer as createHTTPServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { createServer, resolveConfig } from 'vite';
import { expect, it, vi } from 'vitest';

it('keeps each development instance dependency cache inside its isolated run directory', async () => {
  const first = await mkdtemp(join(tmpdir(), 'pluginpocket-vite-first-'));
  const second = await mkdtemp(join(tmpdir(), 'pluginpocket-vite-second-'));
  try {
    vi.stubEnv('PLUGINPOCKET_RUN_DIR', first);
    const a = await resolveConfig(
      { configFile: resolve('vite.config.ts') },
      'serve',
    );
    vi.stubEnv('PLUGINPOCKET_RUN_DIR', second);
    const b = await resolveConfig(
      { configFile: resolve('vite.config.ts') },
      'serve',
    );
    expect(a.cacheDir).toBe(join(first, 'vite-cache'));
    expect(b.cacheDir).toBe(join(second, 'vite-cache'));
    expect(a.cacheDir).not.toBe(b.cacheDir);
  } finally {
    vi.unstubAllEnvs();
    // Vite 冷缓存依赖优化器可能在 close() 后仍有最后一笔写入，ENOTEMPTY 需重试
    const cleanup = (dir: string) =>
      rm(dir, {
        recursive: true,
        force: true,
        maxRetries: 10,
        retryDelay: 200,
      });
    await Promise.all([cleanup(first), cleanup(second)]);
  }
});

it('serves anonymous plugin documents and git files through the real development proxy', async () => {
  const runDir = await mkdtemp(join(tmpdir(), 'pluginpocket-proxy-vite-'));
  vi.stubEnv('PLUGINPOCKET_RUN_DIR', runDir);
  const upstream = createHTTPServer((req, res) => {
    if (req.url === '/api/v1/proxy-origin') {
      res.setHeader('content-type', 'application/json');
      res.end(
        JSON.stringify({
          host: req.headers.host,
          origin: req.headers.origin,
          fetchSite: req.headers['sec-fetch-site'],
        }),
      );
      return;
    }
    res.statusCode = req.url === '/api/v1/plugins/missing' ? 404 : 200;
    res.setHeader('content-type', 'text/plain');
    res.end(`upstream:${req.url}`);
  });
  await new Promise<void>((resolve) =>
    upstream.listen(0, '127.0.0.1', resolve),
  );
  const address = upstream.address();
  if (!address || typeof address === 'string') throw new Error('No port');
  vi.stubEnv('PLUGINPOCKET_API_ORIGIN', `http://127.0.0.1:${address.port}`);
  const vite = await createServer({
    configFile: resolve('vite.config.ts'),
    server: { port: 0, host: '127.0.0.1' },
  });
  try {
    await vite.listen();
    const origin = vite.resolvedUrls?.local[0];
    if (!origin) throw new Error('No Vite URL');
    for (const path of [
      '/api/v1/plugins',
      '/api/v1/plugins/deepwiki',
      '/marketplace.git/HEAD',
      '/marketplace.git/info/refs',
      '/api/v1/plugins/missing',
    ]) {
      const response = await fetch(new URL(path, origin));
      expect(await response.text()).toBe(`upstream:${path}`);
      expect(response.status).toBe(path.endsWith('/missing') ? 404 : 200);
    }
    for (const host of ['localhost', '127.0.0.1']) {
      const url = new URL('/api/v1/proxy-origin', origin);
      url.hostname = host;
      for (const requestOrigin of [url.origin, 'https://evil.example']) {
        const fetchSite =
          requestOrigin === url.origin ? 'same-origin' : 'cross-site';
        const response = await fetch(url, {
          method: 'POST',
          headers: { Origin: requestOrigin, 'Sec-Fetch-Site': fetchSite },
        });
        expect(await response.json()).toEqual({
          host: url.host,
          origin: requestOrigin,
          fetchSite,
        });
      }
    }
  } finally {
    await vite.close();
    await new Promise<void>((resolve, reject) =>
      upstream.close((error) => (error ? reject(error) : resolve())),
    );
    vi.unstubAllEnvs();
    await rm(runDir, {
      recursive: true,
      force: true,
      maxRetries: 10,
      retryDelay: 200,
    });
  }
}, 20000);
