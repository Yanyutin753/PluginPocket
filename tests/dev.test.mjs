import assert from 'node:assert/strict';
import { execFile, spawn } from 'node:child_process';
import { on, once } from 'node:events';
import {
  copyFile,
  mkdir,
  mkdtemp,
  readFile,
  rm,
  writeFile,
} from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';
import { promisify } from 'node:util';

const exec = promisify(execFile);

async function listen() {
  const server = net.createServer();
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  return server;
}

async function port() {
  const server = await listen();
  const value = server.address().port;
  await new Promise((resolve) => server.close(resolve));
  return value;
}

async function fixture(t) {
  const dir = await mkdtemp(join(tmpdir(), 'pluginpocket-dev-'));
  const apiPort = await port();
  const webPort = await port();
  const env = {
    ...process.env,
    PLUGINPOCKET_DATABASE_URL: '',
    PLUGINPOCKET_REDIS_URL: '',
    PLUGINPOCKET_REDIS_NAMESPACE: '',
    PLUGINPOCKET_ADMIN_USERNAME: '',
    PLUGINPOCKET_ADMIN_PASSWORD: '',
    PLUGINPOCKET_RUN_DIR: dir,
    PLUGINPOCKET_ADDR: `127.0.0.1:${apiPort}`,
    PLUGINPOCKET_API_ORIGIN: `http://127.0.0.1:${apiPort}`,
    PLUGINPOCKET_DEV_PORT: String(webPort),
  };
  const make = async (target) => {
    try {
      const result = await exec('make', ['--no-print-directory', target], {
        env,
        timeout: 45_000,
      });
      return { code: 0, output: result.stdout + result.stderr };
    } catch (error) {
      if (error.killed) throw error;
      return { code: error.code, output: error.stdout + error.stderr };
    }
  };
  t.after(async () => {
    await make('down');
    await rm(dir, { recursive: true, force: true });
  });
  return { dir, make, env, apiPort, webPort };
}

const request = (port, path = '/api/v1/health') =>
  fetch(`http://127.0.0.1:${port}${path}`, {
    signal: AbortSignal.timeout(2_000),
  });

test('Make commands read the runtime directory from a local env file', async (t) => {
  const dir = await mkdtemp(join(tmpdir(), 'pluginpocket-env-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await mkdir(join(dir, 'scripts'));
  await mkdir(join(dir, 'custom-run'));
  await copyFile('Makefile', join(dir, 'Makefile'));
  await copyFile('scripts/dev.mjs', join(dir, 'scripts/dev.mjs'));
  await writeFile(join(dir, '.env'), 'PLUGINPOCKET_RUN_DIR=custom-run\n');
  // An inaccessible control socket must be detected in the configured directory.
  await exec(process.execPath, [
    '-e',
    `
    require('node:net').createServer().listen(process.argv[1], () => process.exit(0));
  `,
    join(dir, 'custom-run/dev.sock'),
  ]);
  const env = { ...process.env };
  delete env.PLUGINPOCKET_RUN_DIR;
  await assert.rejects(
    exec('make', ['status'], { cwd: dir, env, timeout: 5_000 }),
    (error) => /Unreachable control socket/.test(error.stderr),
  );
  await writeFile(join(dir, 'custom-run/dev.log'), 'configured-log-entry\n');
  const logs = spawn('make', ['logs'], {
    cwd: dir,
    env,
    detached: true,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const exited = once(logs, 'exit');
  t.after(async () => {
    if (logs.exitCode === null && logs.signalCode === null)
      process.kill(-logs.pid, 'SIGINT');
    await exited;
  });
  let output = '';
  for await (const [chunk] of on(logs.stdout, 'data', {
    signal: AbortSignal.timeout(2_000),
    close: ['end'],
  })) {
    output += chunk.toString();
    if (output.includes('configured-log-entry')) break;
  }
  assert.match(output, /configured-log-entry/);
});

test('background Make commands start ready services, preserve a repeated start, and stop them', {
  timeout: 90_000,
}, async (t) => {
  const f = await fixture(t);
  assert.match((await f.make('status')).output, /stopped/i);
  const started = await f.make('up');
  assert.equal(started.code, 0, started.output);
  for (const port of [f.apiPort, f.webPort]) {
    assert.equal((await (await request(port)).json()).service, 'pluginpocket');
  }
  assert.match(await (await request(f.webPort, '/')).text(), /id="root"/);
  const status = await f.make('status');
  assert.equal(status.code, 0, status.output);
  assert.match(status.output, /running/i);
  assert.match(status.output, new RegExp(String(f.webPort)));
  const log = await readFile(join(f.dir, 'dev.log'), 'utf8');
  assert.match(log, /server listening/);
  const repeated = await f.make('up');
  assert.equal(repeated.code, 0, repeated.output);
  assert.match(repeated.output, /already running/i);
  const restarted = await f.make('restart');
  assert.equal(restarted.code, 0, restarted.output);
  assert.equal(
    (await (await request(f.webPort)).json()).service,
    'pluginpocket',
  );
  const stopped = await f.make('down');
  assert.equal(stopped.code, 0, stopped.output);
  for (const port of [f.apiPort, f.webPort]) {
    await assert.rejects(request(port));
  }
  assert.match((await f.make('status')).output, /stopped/i);
  assert.equal((await f.make('down')).code, 0);
});

test('a blocked development port fails startup without stopping the unrelated listener', {
  timeout: 60_000,
}, async (t) => {
  const f = await fixture(t);
  const blocked = await listen();
  t.after(() => blocked.close());
  f.env.PLUGINPOCKET_DEV_PORT = String(blocked.address().port);
  const started = await f.make('up');
  assert.notEqual(started.code, 0, started.output);
  assert.match(started.output, /in use|EADDRINUSE/);
  assert.match((await f.make('status')).output, /stopped/i);
  await assert.rejects(request(f.apiPort));
  assert.equal(blocked.listening, true);
});

test('an unreachable control socket is not silently replaced', {
  timeout: 60_000,
}, async (t) => {
  const f = await fixture(t);
  // Leave a real stale Unix socket by terminating its owner without cleanup.
  await exec(process.execPath, [
    '-e',
    `
    const net = require('node:net');
    net.createServer().listen(process.argv[1], () => process.exit(0));
  `,
    join(f.dir, 'dev.sock'),
  ]);
  const started = await f.make('up');
  assert.notEqual(started.code, 0, started.output);
  assert.match(started.output, /socket|ECONNREFUSED/);
  await assert.rejects(request(f.apiPort));
});

test('unexpected launcher exit stops its remaining services and clears running status', {
  timeout: 60_000,
}, async (t) => {
  const f = await fixture(t);
  const started = await f.make('up');
  assert.equal(started.code, 0, started.output);
  // Ask this fixture's supervisor for its actual child, independent of pnpm wrappers.
  const statusData = await new Promise((resolve, reject) => {
    const req = http.get(
      { socketPath: join(f.dir, 'dev.sock'), path: '/status' },
      (res) => {
        let body = '';
        res.setEncoding('utf8').on('data', (chunk) => {
          body += chunk;
        });
        res.on('end', () => {
          try {
            resolve(JSON.parse(body));
          } catch (error) {
            reject(error);
          }
        });
        res.on('error', reject);
      },
    );
    req.on('error', reject);
  });
  const launcher = statusData.launcher_pid;
  assert.ok(
    Number.isInteger(launcher) && launcher > 0,
    'supervisor must identify its launcher',
  );
  process.kill(launcher, 'SIGKILL');
  const deadline = Date.now() + 10_000;
  let status;
  do {
    status = await f.make('status');
    if (/stopped/i.test(status.output)) break;
    await delay(100);
  } while (Date.now() < deadline);
  assert.match(status.output, /stopped/i);
  for (const port of [f.apiPort, f.webPort])
    await assert.rejects(request(port));
});

test('a healthy proxy destination cannot hide failure to start the local Go service', {
  timeout: 60_000,
}, async (t) => {
  const f = await fixture(t);
  const upstream = http.createServer((_req, res) => {
    res.setHeader('Content-Type', 'application/json');
    res.end(
      JSON.stringify({
        service: 'pluginpocket',
        status: 'ok',
        version: '0.3.0',
      }),
    );
  });
  upstream.listen(0, '127.0.0.1');
  await once(upstream, 'listening');
  t.after(() => upstream.close());
  f.env.PLUGINPOCKET_API_ORIGIN = `http://127.0.0.1:${upstream.address().port}`;
  // Simulate a delayed compiler failure while Vite can already serve/proxy.
  const bin = join(f.dir, 'bin');
  await mkdir(bin);
  await writeFile(join(bin, 'go'), '#!/bin/sh\nsleep 2\nexit 42\n', {
    mode: 0o700,
  });
  f.env.PATH = `${bin}:${process.env.PATH}`;
  const started = await f.make('up');
  assert.notEqual(started.code, 0, started.output);
  assert.match((await f.make('status')).output, /stopped/i);
  await assert.rejects(request(f.webPort));
});
