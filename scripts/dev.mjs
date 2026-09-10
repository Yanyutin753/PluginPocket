import { fork, spawn } from 'node:child_process';
import { once } from 'node:events';
import { closeSync, mkdirSync, openSync } from 'node:fs';
import http from 'node:http';
import net from 'node:net';
import { resolve } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';

const runDir = resolve(process.env.LOADOUT_RUN_DIR || '.loadout');
const socketPath = resolve(runDir, 'dev.sock');
const logPath = resolve(runDir, 'dev.log');
const addr = process.env.LOADOUT_ADDR || '127.0.0.1:8787';
const api = process.env.LOADOUT_API_ORIGIN || `http://${addr}`;
const web = `http://127.0.0.1:${process.env.LOADOUT_DEV_PORT || 5173}`;

function control(action) {
  return new Promise((resolve, reject) => {
    const req = http.request(
      { socketPath, path: `/${action}`, agent: false },
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
    req.setTimeout(15_000, () =>
      req.destroy(new Error('Development control timed out')),
    );
    req.on('error', (error) => {
      if (error.code === 'ENOENT') resolve(null);
      else if (error.code === 'ECONNREFUSED')
        reject(
          new Error(
            `Unreachable control socket: ${socketPath}; check ${logPath} and remove the socket only after confirming old services have stopped`,
          ),
        );
      else reject(error);
    });
    req.end();
  });
}

async function checkPort(host, port) {
  const listener = net.createServer();
  listener.listen(Number(port), host);
  await once(listener, 'listening');
  await new Promise((resolve) => listener.close(resolve));
}

async function supervise() {
  let state = 'starting';
  let child;
  let closed;
  let stopping;
  const server = http.createServer(async (req, res) => {
    if (req.url === '/down') await stop();
    res.end(
      JSON.stringify({ state, api, web, launcher_pid: child?.pid ?? null }),
    );
  });

  function stop() {
    stopping ??= (async () => {
      state = 'stopping';
      if (child?.pid) {
        const kill = (signal) => {
          try {
            process.kill(-child.pid, signal);
          } catch (error) {
            if (error.code !== 'ESRCH') throw error;
          }
        };
        kill('SIGTERM');
        const watchdog = setTimeout(() => kill('SIGKILL'), 6_000);
        await closed;
        clearTimeout(watchdog);
      }
      state = 'stopped';
      server.close();
    })();
    return stopping;
  }

  try {
    server.listen(socketPath);
    await once(server, 'listening');
    for (const signal of ['SIGINT', 'SIGTERM'])
      process.on(signal, () => void stop());
    process.on('disconnect', () => {
      if (state === 'starting') void stop();
    });
    const split = addr.lastIndexOf(':');
    await checkPort(
      addr.slice(0, split).replace(/^\[|\]$/g, ''),
      addr.slice(split + 1),
    );
    await checkPort('127.0.0.1', new URL(web).port);
    if (stopping) throw new Error('Startup cancelled');
    child = spawn('pnpm', ['dev'], {
      detached: true,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env, LOADOUT_API_ORIGIN: api },
    });
    child.stdout.pipe(process.stdout);
    child.stderr.pipe(process.stderr);
    let failure;
    child.on('error', (error) => {
      failure = error;
      if (state === 'running') void stop();
    });
    child.on('exit', () => {
      failure ??= new Error(`Development service exited; see ${logPath}`);
      if (state === 'running') void stop();
    });
    closed = new Promise((resolve) => child.on('close', resolve));
    const deadline = Date.now() + 30_000;
    while (Date.now() < deadline) {
      if (failure) throw failure;
      if (stopping) throw new Error('Startup cancelled');
      try {
        const local = await fetch(`http://${addr}/api/v1/health`, {
          signal: AbortSignal.timeout(1_000),
        });
        const health = await fetch(`${web}/api/v1/health`, {
          signal: AbortSignal.timeout(1_000),
        });
        const page = await fetch(web, { signal: AbortSignal.timeout(1_000) });
        if (
          local.ok &&
          (await local.json()).service === 'loadout' &&
          health.ok &&
          (await health.json()).service === 'loadout' &&
          page.ok &&
          !failure
        ) {
          state = 'running';
          process.send({ state, api, web });
          return;
        }
      } catch {
        /* Services may still be starting; the deadline bounds retries. */
      }
      await delay(100);
    }
    throw new Error(`Development startup timed out; see ${logPath}`);
  } catch (error) {
    await stop();
    if (process.connected) process.send({ error: error.message });
    process.exitCode = 1;
  }
}

async function main(action) {
  if (action === 'supervise') return supervise();
  if (action === 'logs') {
    const child = spawn('tail', ['-n', '100', '-f', logPath], {
      stdio: 'inherit',
    });
    const [code] = await once(child, 'exit');
    process.exitCode = code ?? 1;
    return;
  }
  if (!['up', 'down', 'status'].includes(action))
    throw new Error('Usage: node scripts/dev.mjs up|down|status|logs');
  const current = await control(action === 'down' ? 'down' : 'status');
  if (action !== 'up') {
    console.log(
      current
        ? `${current.state}: web ${current.web}, API ${current.api}`
        : 'stopped',
    );
    return;
  }
  if (current) {
    if (current.state !== 'running')
      throw new Error(
        `Development services are ${current.state}; retry shortly`,
      );
    console.log(`Already running: ${current.web} (API ${current.api})`);
    return;
  }
  mkdirSync(runDir, { recursive: true, mode: 0o700 });
  const log = openSync(logPath, 'a', 0o600);
  const daemon = fork(new URL(import.meta.url), ['supervise'], {
    detached: true,
    stdio: ['ignore', log, log, 'ipc'],
  });
  closeSync(log);
  const result = await new Promise((resolve, reject) => {
    daemon.once('message', resolve);
    daemon.once('error', reject);
    daemon.once('exit', () =>
      reject(new Error(`Development supervisor exited; see ${logPath}`)),
    );
  });
  daemon.disconnect();
  daemon.unref();
  if (result.error) throw new Error(result.error);
  console.log(
    `Running: ${result.web} (API ${result.api})\nLogs: ${logPath}\nStop: make down`,
  );
}

main(process.argv[2]).catch((error) => {
  console.error(error.message);
  process.exitCode = 1;
});
