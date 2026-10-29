import { defineConfig } from '@playwright/test'

// Docs screenshots (docs/images) against a running Demo Room — `make docs-shots`.
// Not part of the normal e2e run (testDir e2e never sees docs-shots/).
export default defineConfig({
  testDir: 'docs-shots',
  testMatch: 'shots.spec.ts',
  workers: 1,
  timeout: 300_000, // the first test waits for the shop to come up
  expect: { timeout: 15_000 },
  use: { baseURL: process.env.DOCS_URL ?? 'http://127.0.0.1:3200', browserName: 'chromium' },
})
