import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import net from 'node:net';
import { test } from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';

function request(url) {
  return fetch(url, { signal: AbortSignal.timeout(1_000) });
}

async function listen() {
  const server = net.createServer();
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  return server;
}

async function unusedPort() {
  const listener = await listen();
  const port = listener.address().port;
  await new Promise((resolve) => listener.close(resolve));
  return port;
}

async function waitUntil(check, timeout = 15_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (await check()) return;
    await delay(50);
  }
  throw new Error('Timed out waiting for process state');
}

function launch(command, args, env) {
  const child = spawn(command, args, {
    env: { ...process.env, ...env },
    detached: true,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let output = '';
  child.stdout.on('data', (chunk) => {
    output += chunk;
  });
  child.stderr.on('data', (chunk) => {
    output += chunk;
  });
  const exited = once(child, 'exit');
  return { child, exited, output: () => output };
}

function cleanup(proc) {
  try {
    process.kill(-proc.child.pid, 'SIGKILL');
  } catch (error) {
    if (error.code !== 'ESRCH') throw error;
  }
}

test('server executable fails on an occupied address', {
  timeout: 10_000,
}, async (t) => {
  const listener = await listen();
  t.after(() => listener.close());
  const proc = launch('./build/loadout-server', [], {
    LOADOUT_ADDR: `127.0.0.1:${listener.address().port}`,
  });
  t.after(() => cleanup(proc));
  const [code] = await proc.exited;
  assert.equal(code, 1, proc.output());
});

test('SIGTERM shuts down the actual server and releases its port', {
  timeout: 20_000,
}, async (t) => {
  const port = await unusedPort();
  const proc = launch('./build/loadout-server', [], {
    LOADOUT_ADDR: `127.0.0.1:${port}`,
  });
  t.after(() => cleanup(proc));
  await waitUntil(async () => {
    try {
      return (await request(`http://127.0.0.1:${port}/healthz`)).ok;
    } catch {
      return false;
    }
  });
  proc.child.kill('SIGTERM');
  const [code] = await proc.exited;
  assert.equal(code, 0, proc.output());
  await assert.rejects(request(`http://127.0.0.1:${port}/healthz`));
});

test('terminal interruption stops both development services', {
  timeout: 40_000,
}, async (t) => {
  const apiPort = await unusedPort();
  const webPort = await unusedPort();
  const proc = launch('pnpm', ['dev'], {
    LOADOUT_ADDR: `127.0.0.1:${apiPort}`,
    LOADOUT_DEV_PORT: String(webPort),
    LOADOUT_API_ORIGIN: `http://127.0.0.1:${apiPort}`,
  });
  t.after(() => cleanup(proc));
  await waitUntil(async () => {
    try {
      const res = await request(`http://127.0.0.1:${webPort}/api/v1/health`);
      return res.ok && (await res.json()).service === 'loadout';
    } catch {
      return false;
    }
  }, 25_000).catch((error) => {
    throw new Error(`${error.message}\n${proc.output()}`);
  });
  // A terminal delivers Ctrl+C to the foreground process group, including pnpm.
  process.kill(-proc.child.pid, 'SIGINT');
  await proc.exited;
  for (const port of [apiPort, webPort]) {
    await waitUntil(async () => {
      try {
        await request(`http://127.0.0.1:${port}/healthz`);
        return false;
      } catch {
        return true;
      }
    });
  }
});

test('a development subprocess failure shuts down its sibling', {
  timeout: 30_000,
}, async (t) => {
  const blocked = await listen();
  t.after(() => blocked.close());
  const apiPort = await unusedPort();
  const proc = launch('pnpm', ['dev'], {
    LOADOUT_ADDR: `127.0.0.1:${apiPort}`,
    LOADOUT_DEV_PORT: String(blocked.address().port),
  });
  t.after(() => cleanup(proc));
  const [code] = await proc.exited;
  assert.notEqual(code, 0, proc.output());
  assert.match(proc.output(), /already in use/);
  await assert.rejects(request(`http://127.0.0.1:${apiPort}/healthz`));
});
