import { defineConfig } from '@playwright/test'
// Several workers, each with a server and a database of its own (scripts/e2e-servers.sh):
// worker i talks to port 3000+i, so no two share a database, a drawer or a login
// limiter. E2E_WORKERS=1 is the old one-server run.
const workers = Number(process.env.E2E_WORKERS ?? 3)
const index = Number(process.env.TEST_PARALLEL_INDEX ?? 0)

export default defineConfig({
  testDir: 'e2e',
  workers,
  expect: { timeout: 15_000 }, // sign-ins may wait out the login limiter (e2e/fixtures.ts)
  use: { baseURL: `http://localhost:${3000 + index}` },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
    // Shops print from Firefox too: the receipt picture must come out the same there.
    { name: 'firefox', use: { browserName: 'firefox' }, testMatch: /print-image/ },
  ],
  webServer: {
    // Every ported plugin is built in; the tree goes back to core-only straight after.
    // A fresh e2e DB per worker each run: stale rows from an earlier run fail unrelated flows.
    command: `cd .. && E2E_WORKERS=${workers} scripts/e2e-servers.sh`,
    url: 'http://localhost:3000/api/v1/auth/login-info',
    reuseExistingServer: true,
    timeout: 300_000,
  },
})
