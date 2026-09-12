import { readFileSync } from 'node:fs';

export function validateRelease(tag, versions, signingKey) {
  if (!/^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(tag)) {
    throw new Error('Release tag must be a stable version: vMAJOR.MINOR.PATCH');
  }
  if (versions.some((version) => version !== tag.slice(1))) {
    throw new Error(
      'Release version mismatch: update all client manifests before tagging',
    );
  }
  if (!signingKey?.trim()) throw new Error('Release signing key is required');
}

export function validateUpdater(config) {
  const updater = config?.plugins?.updater;
  if (!Array.isArray(updater?.endpoints) || updater.endpoints.length === 0) {
    throw new Error('updater endpoints are required');
  }
  if (!updater.endpoints.every((endpoint) => /^https:\/\//.test(endpoint))) {
    throw new Error('updater endpoints must use https');
  }
  const decoded = Buffer.from(updater.pubkey ?? '', 'base64').toString('utf8');
  if (
    !/^untrusted comment: minisign public key:/m.test(decoded) ||
    !/^RW[A-Za-z0-9+/]{50,}$/m.test(decoded)
  ) {
    throw new Error('updater pubkey must be a base64 minisign public key');
  }
  if (updater.windows?.installMode !== 'passive') {
    throw new Error('updater windows installMode must be passive');
  }
}

if (import.meta.main) {
  const jsonVersion = (path) => JSON.parse(readFileSync(path, 'utf8')).version;
  const cargoVersion = (path) =>
    readFileSync(path, 'utf8').match(/^version = "([^"]+)"$/m)?.[1];
  const tauriConfig = JSON.parse(
    readFileSync('desktop/tauri.conf.json', 'utf8'),
  );
  validateRelease(
    process.env.RELEASE_TAG,
    [
      tauriConfig.version,
      jsonVersion('desktop/ui/package.json'),
      cargoVersion('desktop/Cargo.toml'),
      cargoVersion('cli/Cargo.toml'),
    ],
    process.env.TAURI_SIGNING_PRIVATE_KEY,
  );
  validateUpdater(tauriConfig);
  console.log('Release version and signing configuration present');
}
