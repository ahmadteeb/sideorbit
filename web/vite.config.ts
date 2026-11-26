/// <reference types="vitest/config" />
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()],
  // The CSP allows fonts from this origin only, so never inline them as data: URIs.
  build: { assetsInlineLimit: 0 },
  test: { include: ['src/**/*.test.ts'] }
});
