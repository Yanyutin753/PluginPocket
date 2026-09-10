import '@testing-library/jest-dom/vitest';
import { onlineManager } from '@tanstack/react-query';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

afterEach(() => {
  cleanup();
  onlineManager.setOnline(true);
  vi.unstubAllGlobals();
});
