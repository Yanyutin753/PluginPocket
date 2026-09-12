import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

// Tauri updater 平台键 → 发布产物命名规则（release.yml 上传时带 target 前缀）。
const PLATFORM_ASSETS = [
  {
    key: 'linux-x86_64',
    prefix: 'x86_64-unknown-linux-gnu-',
    suffix: '.AppImage',
  },
  {
    key: 'linux-aarch64',
    prefix: 'aarch64-unknown-linux-gnu-',
    suffix: '.AppImage',
  },
  {
    key: 'darwin-x86_64',
    prefix: 'x86_64-apple-darwin-',
    suffix: '.app.tar.gz',
  },
  {
    key: 'darwin-aarch64',
    prefix: 'aarch64-apple-darwin-',
    suffix: '.app.tar.gz',
  },
  { key: 'windows-x86_64', prefix: 'x86_64-pc-windows-msvc-', suffix: '.exe' },
];

function assetUrl(repo, tag, name) {
  return `https://github.com/${repo}/releases/download/${tag}/${encodeURIComponent(name)}`;
}

export function buildUpdaterManifest(
  files,
  { version, repo, tag, pubDate, notes },
) {
  if (tag !== `v${version}`) {
    throw new Error(
      `updater manifest version mismatch: tag ${tag} does not match version ${version}`,
    );
  }
  const assets = files.filter(
    (file) => !file.name.endsWith('.sig') && !file.name.endsWith('SHA256SUMS'),
  );
  const signatures = new Map(
    files
      .filter((file) => file.name.endsWith('.sig'))
      .map((file) => [file.name.replace(/\.sig$/, ''), file.signature ?? '']),
  );
  const platforms = {};
  for (const { key, prefix, suffix } of PLATFORM_ASSETS) {
    const matches = assets.filter(
      (file) => file.name.startsWith(prefix) && file.name.endsWith(suffix),
    );
    if (matches.length !== 1) {
      throw new Error(
        `updater manifest expects exactly one ${key} asset (${prefix}*${suffix}), found ${matches.length}`,
      );
    }
    const signature = (signatures.get(matches[0].name) ?? '').trim();
    if (!signature) {
      throw new Error(
        `updater manifest requires a signature for ${key} asset ${matches[0].name}`,
      );
    }
    platforms[key] = { signature, url: assetUrl(repo, tag, matches[0].name) };
  }
  return {
    version,
    notes: notes ?? `PluginPocket v${version}`,
    pub_date: pubDate,
    platforms,
  };
}

function parseArgs(argv) {
  const args = Object.fromEntries(
    argv
      .filter((_, index) => index % 2 === 0)
      .map((flag, index) => [flag.replace(/^--/, ''), argv[index * 2 + 1]]),
  );
  for (const required of ['assets', 'tag', 'repo', 'out']) {
    if (!args[required]) throw new Error(`missing required --${required}`);
  }
  return args;
}

if (import.meta.main) {
  const { assets: dir, tag, repo, out } = parseArgs(process.argv.slice(2));
  const files = readdirSync(dir).map((name) => {
    if (!name.endsWith('.sig')) return { name };
    return { name, signature: readFileSync(join(dir, name), 'utf8') };
  });
  const manifest = buildUpdaterManifest(files, {
    version: tag.replace(/^v/, ''),
    repo,
    tag,
    pubDate: new Date().toISOString(),
  });
  writeFileSync(out, `${JSON.stringify(manifest, null, 2)}\n`);
  console.log(`updater manifest written to ${out} for ${tag}`);
}
