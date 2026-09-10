import '@testing-library/jest-dom/vitest';
import { onlineManager } from '@tanstack/react-query';
import { cleanup, configure } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

// Cold lazy-route transforms share CPU with the full harness. Keep assertions
// condition-based while allowing them to finish within Vitest's test deadline.
configure({ asyncUtilTimeout: 3000 });

afterEach(() => {
  cleanup();
  onlineManager.setOnline(true);
  vi.unstubAllGlobals();
});

// jsdom omits geometry APIs used by the real Radix focus and popover primitives.
Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
  configurable: true,
  value: vi.fn(),
});
Object.defineProperty(Element.prototype, 'hasPointerCapture', {
  configurable: true,
  value: () => false,
});
Object.defineProperty(Element.prototype, 'setPointerCapture', {
  configurable: true,
  value: vi.fn(),
});
Object.defineProperty(Element.prototype, 'releasePointerCapture', {
  configurable: true,
  value: vi.fn(),
});
