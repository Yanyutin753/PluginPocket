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

if (import.meta.main) {
  const jsonVersion = (path) => JSON.parse(readFileSync(path, 'utf8')).version;
  const cargoVersion = (path) =>
    readFileSync(path, 'utf8').match(/^version = "([^"]+)"$/m)?.[1];
  validateRelease(
    process.env.RELEASE_TAG,
    [
      jsonVersion('desktop/tauri.conf.json'),
      jsonVersion('desktop/ui/package.json'),
      cargoVersion('desktop/Cargo.toml'),
      cargoVersion('cli/Cargo.toml'),
    ],
    process.env.TAURI_SIGNING_PRIVATE_KEY,
  );
  console.log('Release version and signing configuration present');
}
