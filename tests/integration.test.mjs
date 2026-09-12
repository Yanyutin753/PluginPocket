import assert from 'node:assert/strict';
import { execFile, spawn } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, rm } from 'node:fs/promises';
import net from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';
import { promisify } from 'node:util';

const exec = promisify(execFile);
let origin;
let server;
let exited;
let logs = '';

before(
  async () => {
    const socket = net.createServer();
    socket.listen(0, '127.0.0.1');
    await once(socket, 'listening');
    const port = socket.address().port;
    await new Promise((resolve) => socket.close(resolve));
    origin = `http://127.0.0.1:${port}`;
    server = spawn('./build/pluginpocket-server', [], {
      env: {
        ...process.env,
        PLUGINPOCKET_DATABASE_URL: '',
        PLUGINPOCKET_REDIS_URL: '',
        PLUGINPOCKET_REDIS_NAMESPACE: '',
        PLUGINPOCKET_ADMIN_USERNAME: '',
        PLUGINPOCKET_ADMIN_PASSWORD: '',
        PLUGINPOCKET_ADDR: `127.0.0.1:${port}`,
        PLUGINPOCKET_WEB_DIR: 'web/dist',
      },
      stdio: ['ignore', 'ignore', 'pipe'],
    });
    server.stderr.on('data', (chunk) => {
      logs += chunk;
    });
    exited = once(server, 'exit');
    // Attach immediately so a spawn failure cannot become an unhandled rejection.
    exited.catch(() => {});
    const deadline = Date.now() + 10_000;
    while (Date.now() < deadline) {
      if (server.exitCode !== null) throw new Error(`Server exited: ${logs}`);
      try {
        if ((await request('/healthz')).ok) return;
      } catch {
        /* Startup is bounded by the deadline. */
      }
      await delay(25);
    }
    throw new Error(`Server did not become ready: ${logs}`);
  },
  { timeout: 12_000 },
);

after(async () => {
  if (!server?.pid || server.exitCode !== null) return;
  server.kill('SIGTERM');
  const watchdog = setTimeout(() => server.kill('SIGKILL'), 6_000);
  try {
    await exited;
  } finally {
    clearTimeout(watchdog);
  }
});

function request(path, options) {
  return fetch(`${origin}${path}`, {
    ...options,
    signal: AbortSignal.timeout(3_000),
  });
}

test('production health API exposes the canonical contract without caching', async () => {
  for (const path of ['/healthz', '/api/v1/health']) {
    const response = await request(path);
    assert.equal(response.status, 200);
    assert.match(response.headers.get('content-type'), /application\/json/);
    assert.equal(response.headers.get('cache-control'), 'no-store');
    assert.deepEqual(await response.json(), {
      status: 'ok',
      service: 'pluginpocket',
      version: '0.3.0',
    });
    const head = await request(path, { method: 'HEAD' });
    assert.equal(head.status, 200);
    assert.equal(await head.text(), '');
  }
});

test('API and missing asset failures never fall through to the SPA', async () => {
  for (const path of ['/api/v1/missing', '/mcp', '/assets/missing.js']) {
    const response = await request(path);
    assert.equal(response.status, 404);
    assert.match(response.headers.get('content-type'), /application\/json/);
  }
  const response = await request('/api/v1/health', { method: 'POST' });
  assert.equal(response.status, 405);
  assert.equal(response.headers.get('allow'), 'GET, HEAD');
});

test('Go serves built React HTML, SPA routes and their actual assets', async () => {
  const response = await request('/');
  assert.equal(response.status, 200);
  const html = await response.text();
  assert.match(html, /id="root"/);
  const route = await request('/dashboard');
  assert.equal(route.status, 200);
  assert.equal(await route.text(), html);
  const assets = [...html.matchAll(/(?:src|href)="(\/assets\/[^" ]+)"/g)].map(
    (match) => match[1],
  );
  assert.ok(assets.some((asset) => asset.endsWith('.js')));
  assert.ok(assets.some((asset) => asset.endsWith('.css')));
  for (const asset of assets) {
    const file = await request(asset);
    assert.equal(file.status, 200);
    assert.ok((await file.arrayBuffer()).byteLength > 0);
  }
});

test('real Rust release CLI checks the same Go service', {
  timeout: 10_000,
}, async (t) => {
  const home = await mkdtemp(join(tmpdir(), 'pluginpocket-doctor-'));
  t.after(() => rm(home, { recursive: true, force: true }));
  const { stdout, stderr } = await exec(
    './cli/target/release/pluginpocket',
    ['doctor', '--server', origin],
    {
      timeout: 8_000,
      env: {
        ...process.env,
        PLUGINPOCKET_DATABASE_URL: '',
        PLUGINPOCKET_REDIS_URL: '',
        PLUGINPOCKET_REDIS_NAMESPACE: '',
        PLUGINPOCKET_ADMIN_USERNAME: '',
        PLUGINPOCKET_ADMIN_PASSWORD: '',
        HOME: home,
        USERPROFILE: home,
        PLUGINPOCKET_CONFIG: join(home, 'config.json'),
        NO_PROXY: '*',
      },
    },
  );
  assert.match(stdout, /PluginPocket 0\.3\.0 is reachable/);
  assert.equal(stderr, '');
});
