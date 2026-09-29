import { test as base } from '@playwright/test'

export { expect, type Page } from '@playwright/test'

// Every spec signs in from one IP, so the suite outruns the login limiter
// (burst 10, +1 per 2s; unlock shares it). Wait out a 429 instead of failing.
export const test = base.extend({
  page: async ({ page }, provide) => {
    await page.route(/\/api\/v1\/auth\/(login|unlock)$/, async (route) => {
      try {
        for (let i = 0; ; i++) {
          const res = await route.fetch()
          if (res.status() !== 429 || i === 10) return await route.fulfill({ response: res })
          await new Promise((r) => setTimeout(r, 2100))
        }
      } catch {
        // the test ended mid-retry
      }
    })
    await provide(page)
  },
})
