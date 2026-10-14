import { defineConfig } from '@playwright/test'
export default defineConfig({
  testDir: 'e2e',
  // One worker: every spec shares one database, one drawer and one login limiter.
  workers: 1,
  expect: { timeout: 15_000 }, // sign-ins may wait out the login limiter (e2e/fixtures.ts)
  use: { baseURL: 'http://localhost:3100' },
  webServer: {
    // Every ported plugin is built in; the tree goes back to core-only straight after.
    // A fresh e2e DB each run: stale rows from an earlier run fail unrelated flows.
    command: 'cd .. && make e2e-seed && make plugins P=all && make build && make core-only && set -a && . ./.env && set +a && ./bin/karots-suite',
    url: 'http://localhost:3100/api/v1/auth/login-info',
    reuseExistingServer: true,
    timeout: 300_000,
  },
})
