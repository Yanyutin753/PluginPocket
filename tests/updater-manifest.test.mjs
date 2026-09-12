import assert from 'node:assert/strict';
import test from 'node:test';
import { buildUpdaterManifest } from '../scripts/build-updater-manifest.mjs';

const options = {
  version: '1.2.3',
  repo: 'Yanyutin753/PluginPocket',
  tag: 'v1.2.3',
  pubDate: '2026-09-12T00:00:00.000Z',
};

function signed(name, signature) {
  return [{ name }, { name: `${name}.sig`, signature }];
}

const files = [
  ...signed(
    'x86_64-unknown-linux-gnu-PluginPocket_1.2.3_amd64.AppImage',
    'sig:linux-x64',
  ),
  ...signed(
    'aarch64-unknown-linux-gnu-PluginPocket_1.2.3_arm64.AppImage',
    'sig:linux-arm64',
  ),
  ...signed('aarch64-apple-darwin-PluginPocket.app.tar.gz', 'sig:darwin-arm64'),
  ...signed('x86_64-apple-darwin-PluginPocket.app.tar.gz', 'sig:darwin-x64'),
  ...signed(
    'x86_64-pc-windows-msvc-PluginPocket_1.2.3_x64-setup.exe',
    'sig:windows',
  ),
  { name: 'x86_64-unknown-linux-gnu-PluginPocket_1.2.3_amd64.deb' },
  { name: 'x86_64-pc-windows-msvc-PluginPocket_1.2.3_x64_en-US.msi' },
  { name: 'x86_64-unknown-linux-gnu-SHA256SUMS' },
];

test('manifest maps every updater platform to a signed release asset', () => {
  const manifest = buildUpdaterManifest(files, options);
  assert.deepEqual(Object.keys(manifest.platforms).sort(), [
    'darwin-aarch64',
    'darwin-x86_64',
    'linux-aarch64',
    'linux-x86_64',
    'windows-x86_64',
  ]);
  const linux = manifest.platforms['linux-x86_64'];
  assert.equal(
    linux.url,
    'https://github.com/Yanyutin753/PluginPocket/releases/download/v1.2.3/x86_64-unknown-linux-gnu-PluginPocket_1.2.3_amd64.AppImage',
  );
  assert.equal(linux.signature, 'sig:linux-x64');
  assert.equal(manifest.version, '1.2.3');
  assert.equal(manifest.pub_date, '2026-09-12T00:00:00.000Z');
  const windows = manifest.platforms['windows-x86_64'];
  assert.match(windows.url, /\.exe$/);
  assert.equal(windows.signature, 'sig:windows');
});

test('manifest refuses a platform asset without a usable signature', () => {
  const unsigned = files.filter((file) => !file.name.includes('darwin'));
  assert.throws(
    () => buildUpdaterManifest(unsigned, options),
    /darwin-(x86_64|aarch64)/,
  );
  const emptySig = files.map((file) =>
    file.name.includes('windows') && file.signature
      ? { ...file, signature: ' ' }
      : file,
  );
  assert.throws(
    () => buildUpdaterManifest(emptySig, options),
    /windows-x86_64[\s\S]*signature|signature[\s\S]*windows-x86_64/i,
  );
});

test('manifest refuses a tag that disagrees with the manifest version', () => {
  assert.throws(
    () => buildUpdaterManifest(files, { ...options, tag: 'v1.2.4' }),
    /tag.*version|version.*tag/i,
  );
});
