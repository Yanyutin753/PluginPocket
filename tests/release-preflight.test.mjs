import assert from 'node:assert/strict';
import test from 'node:test';
import {
  validateRelease,
  validateUpdater,
} from '../scripts/release-preflight.mjs';

const validUpdaterConfig = {
  plugins: {
    updater: {
      endpoints: [
        'https://github.com/Yanyutin753/PluginPocket/releases/latest/download/latest.json',
      ],
      pubkey: Buffer.from(
        'untrusted comment: minisign public key: E09FD4AFEB8E749C\nRWSgdI7rr9Sf4HAX6sUWqzQfHt9u/rvheRqbCpDYurSMI8Gh7JJysLD\n',
      ).toString('base64'),
      windows: { installMode: 'passive' },
    },
  },
};

test('release refuses branches and malformed versions', () => {
  for (const tag of ['main', 'v1.2', 'v1.2.3;echo', 'v01.2.3']) {
    assert.throws(
      () => validateRelease(tag, ['1.2.3'], 'key'),
      /stable version/,
    );
  }
});
test('release refuses version drift across packaged clients', () => {
  assert.throws(
    () => validateRelease('v1.2.3', ['1.2.3', '1.2.2'], 'key'),
    /version mismatch/,
  );
});
test('release refuses missing signing identity', () => {
  assert.throws(() => validateRelease('v1.2.3', ['1.2.3'], ''), /signing key/);
});
test('release accepts matching stable versions with signing identity', () => {
  assert.doesNotThrow(() =>
    validateRelease('v1.2.3', ['1.2.3', '1.2.3'], 'key'),
  );
});

test('updater config requires reachable endpoints and a minisign pubkey', () => {
  assert.throws(
    () => validateUpdater({ plugins: { updater: { pubkey: 'a2V5' } } }),
    /endpoints/,
  );
  assert.throws(
    () =>
      validateUpdater({
        plugins: {
          updater: {
            endpoints: [
              'http://github.com/Yanyutin753/PluginPocket/releases/latest/download/latest.json',
            ],
            pubkey: 'a2V5',
          },
        },
      }),
    /https/,
  );
  assert.throws(
    () =>
      validateUpdater({
        plugins: {
          updater: { ...validUpdaterConfig.plugins.updater, pubkey: '###' },
        },
      }),
    /pubkey/,
  );
  assert.throws(
    () =>
      validateUpdater({
        plugins: {
          updater: {
            ...validUpdaterConfig.plugins.updater,
            pubkey: Buffer.from('not a minisign key').toString('base64'),
          },
        },
      }),
    /pubkey/,
  );
  assert.throws(
    () =>
      validateUpdater({
        plugins: {
          updater: {
            ...validUpdaterConfig.plugins.updater,
            windows: { installMode: 'basicUi' },
          },
        },
      }),
    /installMode/,
  );
  assert.doesNotThrow(() => validateUpdater(validUpdaterConfig));
});
