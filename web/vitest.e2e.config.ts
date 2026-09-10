import { fileURLToPath, URL } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const origin = process.env.LOADOUT_E2E_ORIGIN;
if (!origin)
  throw new Error(
    'LOADOUT_E2E_ORIGIN must point to the isolated Go test server',
  );

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  test: {
    include: ['src/e2e/**/*.e2e.tsx'],
    environment: 'jsdom',
    environmentOptions: { jsdom: { url: origin } },
    setupFiles: ['./src/e2e/setup.ts'],
    testTimeout: 45_000,
    hookTimeout: 10_000,
    fileParallelism: false,
  },
});
