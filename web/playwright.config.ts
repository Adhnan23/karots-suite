import { defineConfig } from '@playwright/test'
export default defineConfig({
  testDir: 'e2e',
  use: { baseURL: 'http://localhost:3100' },
  webServer: {
    command: 'cd .. && make build && set -a && . ./.env && set +a && ./bin/karots-suite',
    url: 'http://localhost:3100/api/v1/auth/login-info',
    reuseExistingServer: true,
    timeout: 180_000,
  },
})
