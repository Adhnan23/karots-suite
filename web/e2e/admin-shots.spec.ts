import { test, type Page } from './fixtures'

// UI-gate screenshots for admin pages (not a behaviour test). Needs `make e2e-seed`.
//   SHOTS=m2-1 SHOTS_PATH=/admin/units bunx playwright test e2e/admin-shots.spec.ts
// SHOTS_PREP (optional) names a prep step below run after the page loads.
// Output: web/shots/<gate>/<name>-<palette>-<mode>-<width>.png
const gate = process.env.SHOTS
const path = process.env.SHOTS_PATH ?? '/admin'
const prepKey = process.env.SHOTS_PREP ?? ''
const name = prepKey || path.replace(/^\/admin\/?/, '').replace(/\//g, '-') || 'home'
test.skip(!gate, 'set SHOTS=<gate> to take UI-gate screenshots')
test.describe.configure({ mode: 'serial' })

async function login(page: Page, phone: string, pin: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill(pin)
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(admin|till)/)
}

const prep: Record<string, (page: Page) => Promise<void>> = {
  'unit-form': async (page) => {
    await page.getByRole('button', { name: /New unit/ }).click()
    await page.getByLabel('Name').fill('Kilogram')
  },
  shelf: async (page) => {
    await page.getByRole('button', { name: 'Setup' }).first().click()
  },
}

for (const palette of ['studio', 'night']) {
  for (const mode of ['light', 'dark']) {
    for (const width of [1366, 360]) {
      test(`${palette} ${mode} ${width}`, async ({ page }) => {
        await page.setViewportSize({ width, height: width < 768 ? 780 : 768 })
        await login(page, '0000000001', process.env.POS_SYSTEM_PIN ?? '2273')
        const cur = await (await page.request.get('/api/v1/system/appearance')).json()
        await page.request.put('/api/v1/system/appearance', { data: { ...cur, palette } })
        await page.context().clearCookies()
        await page.addInitScript((m) => localStorage.setItem('ks-mode', m), mode)
        await login(page, '0770000001', '1234')
        await page.goto(path)
        await page.waitForLoadState('networkidle')
        await prep[prepKey]?.(page)
        await page.waitForTimeout(300)
        await page.screenshot({ path: `shots/${gate}/${name}-${palette}-${mode}-${width}.png` })
      })
    }
  }
}
