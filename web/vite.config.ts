import { resolve } from 'node:path';
import { fileURLToPath, URL } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  cacheDir: process.env.LOADOUT_RUN_DIR
    ? resolve(process.env.LOADOUT_RUN_DIR, 'vite-cache')
    : fileURLToPath(new URL('./node_modules/.vite', import.meta.url)),
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    port: Number(process.env.LOADOUT_DEV_PORT ?? 5173),
    proxy: {
      '^/marketplace\\.git(?:/|$|\\?)':
        process.env.LOADOUT_API_ORIGIN ?? 'http://127.0.0.1:8787',
      '/api': {
        target: process.env.LOADOUT_API_ORIGIN ?? 'http://127.0.0.1:8787',
        changeOrigin: false,
      },
      '/healthz': process.env.LOADOUT_API_ORIGIN ?? 'http://127.0.0.1:8787',
      '/readyz': process.env.LOADOUT_API_ORIGIN ?? 'http://127.0.0.1:8787',
      '/mcp': process.env.LOADOUT_API_ORIGIN ?? 'http://127.0.0.1:8787',
    },
  },
  test: {
    maxWorkers: 4,
    include: ['src/**/*.test.{ts,tsx}'],
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    restoreMocks: true,
  },
});
