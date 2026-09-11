import '@testing-library/jest-dom/vitest';
import '../test/editor-geometry';
import { cleanup, configure } from '@testing-library/react';
import { afterEach, beforeEach } from 'vitest';

// Vitest exposes its jsdom instance. Use its real cookie implementation,
// including HttpOnly, expiry and path rules; never fabricate API responses.
declare const jsdom: {
  cookieJar: {
    getCookieStringSync(url: string): string;
    setCookieSync(cookie: string, url: string): unknown;
    removeAllCookiesSync(): void;
  };
};

export const networkFetch = globalThis.fetch;
export const origin = window.location.origin;
export const sessionCookie = () => jsdom.cookieJar.getCookieStringSync(origin);

configure({ asyncUtilTimeout: 5_000 });
beforeEach(() => {
  jsdom.cookieJar.removeAllCookiesSync();
  globalThis.fetch = async (input, init) => {
    const url = new URL(
      input instanceof Request ? input.url : String(input),
      origin,
    );
    if (url.origin !== origin)
      throw new Error('E2E only permits its isolated API origin');
    const request = new Request(input instanceof Request ? input : url, init);
    const headers = new Headers(request.headers);
    if (request.credentials !== 'omit') {
      const cookie = jsdom.cookieJar.getCookieStringSync(url.href);
      if (cookie) headers.set('Cookie', cookie);
    }
    if (!['GET', 'HEAD'].includes(request.method))
      headers.set('Origin', origin);
    const response = await networkFetch(
      new Request(request, {
        headers,
        signal: AbortSignal.any([request.signal, AbortSignal.timeout(10_000)]),
      }),
    );
    if (request.credentials !== 'omit') {
      for (const cookie of response.headers.getSetCookie()) {
        jsdom.cookieJar.setCookieSync(cookie, url.href);
      }
    }
    return response;
  };
});
afterEach(() => {
  cleanup();
  globalThis.fetch = networkFetch;
  jsdom.cookieJar.removeAllCookiesSync();
});
