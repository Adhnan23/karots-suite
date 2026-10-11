import { expect, test, type Page } from '@playwright/test'

// UI-gate screenshots (not a behaviour test). Needs `make e2e-seed` data.
//   SHOTS=gate1 SHOTS_SCENE=closed bunx playwright test e2e/till-shots.spec.ts
// Scenes: closed (open-till overlay) · sale (mid-sale: lines, lot, customer, tender) · done (sale complete).
// Output: web/shots/<gate>/<scene>-<palette>-<mode>-<width>.png
const gate = process.env.SHOTS
const scene = process.env.SHOTS_SCENE ?? 'sale'
test.skip(!gate, 'set SHOTS=<gate> to take UI-gate screenshots')
test.describe.configure({ mode: 'serial' })

async function login(page: Page, phone: string, pin: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill(pin)
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(admin|till)/)
}

async function setPalette(page: Page, palette: string) {
  await login(page, '0000000001', process.env.POS_SYSTEM_PIN ?? '2273')
  const cur = await (await page.request.get('/api/v1/system/appearance')).json()
  if (scene === 'clearance') {
    // A live clearance markdown on milk (the plugin must be built in).
    const milk = (await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()).id
    expect((await page.request.post(`/api/v1/x/clearance/${milk}/approve`, { data: { percent: '10' } })).ok()).toBe(true)
  }
  expect((await page.request.put('/api/v1/system/appearance', { data: { ...cur, palette } })).ok()).toBe(true)
  await page.context().clearCookies()
}

async function setScene(page: Page) {
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  const open = !!st.summary.session
  if (scene === 'closed') {
    if (open) await page.request.post('/api/v1/till/drawer/close', { data: { breakdown: {}, destLockerId: 0 } })
    await page.reload()
    return
  }
  if (!open) {
    await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
    await page.reload()
  }
  const s = page.getByLabel('Scan a barcode or search products')
  if (scene === 'clearance') {
    await s.fill('E2E Milk')
    await page.getByText('Clearance -10%').first().waitFor()
    await page.screenshot({ path: `shots/${gate}/clearance-badge-${await page.evaluate(() => document.documentElement.dataset.palette ?? '')}-${await page.evaluate(() => localStorage.getItem('ks-mode'))}-${page.viewportSize()!.width}.png` })
    await s.fill('4790000000011')
    await s.press('Enter')
    await page.waitForTimeout(400)
    return
  }
  for (const code of ['4790000000011', '4790000000011']) {
    await s.fill(code)
    await s.press('Enter')
  }
  const amount = page.getByLabel('Amount tendered')
  if (scene === 'done') {
    // No lot soap here: each completed run sells one, and 8 runs drain its 5-unit first lot.
    await amount.fill('1000')
    await amount.press('Enter')
    await amount.press('Enter')
    await page.getByRole('dialog', { name: /complete/ }).waitFor()
    return
  }
  await s.fill('4790000000028')
  await s.press('Enter')
  await page.getByRole('dialog', { name: 'Which price?' }).getByRole('button', { name: /120\.00/ }).click()
  await page.getByRole('button', { name: /Walk-in customer/ }).click()
  await page.getByLabel('Search customers').fill('E2E Credit')
  await page.getByLabel('Search customers').press('Enter')
  await page.getByRole('button', { name: 'Cash', exact: true }).click()
  await amount.fill('500')
}

for (const palette of ['studio', 'night']) {
  for (const mode of ['light', 'dark']) {
    for (const width of [1366, 1024]) {
      test(`${palette} ${mode} ${width}`, async ({ browser }) => {
        const ctx = await browser.newContext({ viewport: { width, height: 768 } })
        const page = await ctx.newPage()
        await setPalette(page, palette)
        await page.addInitScript((m) => localStorage.setItem('ks-mode', m), mode)
        await login(page, '0770000002', '1234')
        await page.goto('/till')
        await page.waitForLoadState('networkidle')
        await setScene(page)
        await page.screenshot({ path: `shots/${gate}/${scene === 'clearance' ? 'till-clearance' : scene}-${palette}-${mode}-${width}.png` })
        await ctx.close()
      })
    }
  }
}
