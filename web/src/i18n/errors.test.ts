import { describe, expect, it } from 'vitest';
import type { Locale } from '../i18n';
import contract from './error-codes.json';
import { errorMessages, errorText } from './errors';

const backendCodes = (contract as { code: string }[]).map(
  (entry) => entry.code,
);
const localCodes = ['invalid_json', 'unknown'];

describe('error message dictionary', () => {
  it('covers every backend contract code in both locales', () => {
    const missing = backendCodes.filter(
      (code) => !errorMessages[code]?.zh || !errorMessages[code]?.en,
    );
    expect(missing, `codes without zh/en copy: ${missing.join(', ')}`).toEqual(
      [],
    );
  });

  it('holds no phantom codes beyond the known frontend-local ones', () => {
    const phantoms = Object.keys(errorMessages).filter(
      (code) => !backendCodes.includes(code) && !localCodes.includes(code),
    );
    expect(
      phantoms,
      `codes the backend never emits: ${phantoms.join(', ')}`,
    ).toEqual([]);
  });

  it.each(['zh-CN', 'en'] as Locale[])(
    'falls back to the generic message for unknown codes in %s',
    (locale) => {
      expect(errorText('no_such_code', locale)).toBe(
        errorMessages.unknown[locale === 'en' ? 'en' : 'zh'],
      );
    },
  );

  it('resolves a registered code per locale', () => {
    expect(errorText('last_admin', 'zh-CN')).toBe(errorMessages.last_admin.zh);
    expect(errorText('last_admin', 'en')).toBe(errorMessages.last_admin.en);
  });
});
