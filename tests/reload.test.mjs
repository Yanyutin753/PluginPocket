import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { copyFile, mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import net from 'node:net';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { test } from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';

test('dev-server reloads Go changes, recovers from build errors, and releases its port', {
  timeout: 60_000,
}, async (t) => {
  const dir = await mkdtemp(join(tmpdir(), 'pluginpocket-reload-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await copyFile('Makefile', join(dir, 'Makefile'));
  // Before hot reload exists, the same Make target still starts the fixture.
  await copyFile('.air.toml', join(dir, '.air.toml')).catch((error) => {
    if (error.code !== 'ENOENT') throw error;
  });
  await mkdir(join(dir, 'server/cmd/pluginpocket-server'), { recursive: true });
  await writeFile(
    join(dir, 'server/go.mod'),
    'module reloadfixture\n\ngo 1.26\n',
  );
  const source = join(dir, 'server/cmd/pluginpocket-server/main.go');
  const update = (value) =>
    writeFile(
      source,
      `package main
import ("net/http"; "os")
func main() {
  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("${value}")) })
  if err := http.ListenAndServe(os.Getenv("PLUGINPOCKET_ADDR"), nil); err != nil { panic(err) }
}
`,
    );
  await update('before');
  const listener = net.createServer().listen(0, '127.0.0.1');
  await once(listener, 'listening');
  const addr = `127.0.0.1:${listener.address().port}`;
  await new Promise((resolve) => listener.close(resolve));
  const child = spawn(
    'make',
    ['dev-server', `AIR=${resolve('build/tools/air-1.67.4/air')}`],
    {
      cwd: dir,
      env: { ...process.env, PLUGINPOCKET_ADDR: addr },
      detached: true,
      stdio: ['ignore', 'pipe', 'pipe'],
    },
  );
  const exited = once(child, 'exit');
  let output = '';
  for (const stream of [child.stdout, child.stderr])
    stream.on('data', (chunk) => {
      output += chunk;
    });
  t.after(async () => {
    try {
      process.kill(-child.pid, 'SIGKILL');
    } catch (error) {
      if (error.code !== 'ESRCH') throw error;
    }
    await exited;
  });
  const body = async () => {
    try {
      return await (
        await fetch(`http://${addr}`, { signal: AbortSignal.timeout(500) })
      ).text();
    } catch {
      return null;
    }
  };
  async function expectBody(expected) {
    const deadline = Date.now() + 12_000;
    let actual;
    do {
      actual = await body();
      if (actual === expected) return;
      await delay(100);
    } while (Date.now() < deadline);
    assert.equal(actual, expected, output);
  }
  await expectBody('before');
  await update('after');
  await expectBody('after');
  await writeFile(source, 'package main\nfunc main() { invalid Go }\n');
  await expectBody(null);
  await update('recovered');
  await expectBody('recovered');
  process.kill(-child.pid, 'SIGINT');
  await exited;
  await expectBody(null);
});
