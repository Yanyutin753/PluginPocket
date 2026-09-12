import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

afterEach(cleanup);
Object.defineProperty(window.navigator, 'language', {
  value: 'zh-CN',
  configurable: true,
});
