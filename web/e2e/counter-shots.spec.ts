import { test, type Page } from './fixtures'

// UI-gate screenshots for the counter tabs (not a behaviour test). Needs `make e2e-seed`.
//   SHOTS=m1b-2 SHOTS_TAB=expenses bunx playwright test e2e/counter-shots.spec.ts
// SHOTS_PREP (optional) names a prep step below that fills the tab before the shot.
// Output: web/shots/<gate>/<tab>-<palette>-<mode>-<width>.png
const gate = process.env.SHOTS
const tab = process.env.SHOTS_TAB ?? 'expenses'
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
  expenses: async (page) => {
    await page.getByRole('button', { name: 'Transport', exact: true }).click()
    await page.getByLabel('Amount').fill('350')
    await page.getByLabel('Note').fill('Delivery tuk')
    await page.getByRole('button', { name: /Record expense/ }).click()
    await page.getByText('Delivery tuk').waitFor()
    await page.getByRole('button', { name: 'Electricity', exact: true }).click()
    await page.getByLabel('Amount').fill('1200')
  },
  'write-off': async (page) => {
    await page.getByLabel('Find a product').fill('Lot Soap')
    await page.getByRole('option').first().click()
    await page.getByRole('radio', { name: /Shop use/ }).click()
    await page.getByRole('radio', { name: /120\.00/ }).click()
    await page.getByLabel('Quantity').fill('2')
    await page.getByLabel('Note').fill('Washing the counter')
  },
  labels: async (page) => {
    await page.getByLabel('Find a product').fill('Milk')
    await page.getByRole('option', { name: /E2E Milk\b(?! Crate)/ }).click()
    await page.getByRole('button', { name: '24', exact: true }).click()
    await page.getByRole('button', { name: /40 × 30/ }).click()
  },
  convert: async (page) => {
    await page.getByRole('button', { name: /Milk Crate/ }).click()
    await page.getByLabel('Packs to open').fill('2')
  },
}

for (const palette of ['studio', 'night']) {
  for (const mode of ['light', 'dark']) {
    for (const width of [1366, 1024]) {
      test(`${palette} ${mode} ${width}`, async ({ page }) => {
        await page.setViewportSize({ width, height: 768 })
        await login(page, '0000000001', process.env.POS_SYSTEM_PIN ?? '2273')
        const cur = await (await page.request.get('/api/v1/system/appearance')).json()
        await page.request.put('/api/v1/system/appearance', { data: { ...cur, palette } })
        await page.context().clearCookies()
        await page.addInitScript((m) => localStorage.setItem('ks-mode', m), mode)
        await login(page, '0770000001', '1234') // admin: sees every tab
        const st = await (await page.request.get('/api/v1/till/drawer')).json()
        if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 4 }, sourceLockerId: 0 } })
        await page.goto(`/till/${tab}`)
        await page.waitForLoadState('networkidle')
        await prep[tab]?.(page)
        await page.waitForTimeout(300)
        await page.screenshot({ path: `shots/${gate}/${tab}-${palette}-${mode}-${width}.png` })
      })
    }
  }
}
