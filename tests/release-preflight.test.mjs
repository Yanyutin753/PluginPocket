import assert from 'node:assert/strict';
import test from 'node:test';
import { validateRelease } from '../scripts/release-preflight.mjs';

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
