import { expect, test, type Page } from '@playwright/test'
import { alternativesData } from './shot-prep-alternatives'

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
  if (scene === 'requests' && !(await (await page.request.get('/api/v1/x/requests/similar?q=umbrella')).json()).length) {
    // An open request asked twice and a won't-stock one (the plugin must be built in).
    const add = async (note: string) => (await (await page.request.post('/api/v1/x/requests', { data: { note, name: '', phone: '' } })).json()).id
    const open = await add('Umbrella red, large')
    await page.request.post(`/api/v1/x/requests/${open}/asks`, { data: { name: 'Mala', phone: '' } })
    await page.request.post(`/api/v1/x/requests/${await add('Umbrella with torch')}/decline`, { data: { reason: 'supplier stopped making them' } })
  }
  if (scene === 'alternatives') await alternativesData(page) // the plugin must be built in
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
  if (scene === 'requests') {
    const shot = async (name: string) =>
      page.screenshot({ path: `shots/${gate}/${name}-${await page.evaluate(() => document.documentElement.dataset.palette ?? '')}-${await page.evaluate(() => localStorage.getItem('ks-mode'))}-${page.viewportSize()!.width}.png` })
    await s.fill('zzq no such thing')
    await s.press('Enter')
    await page.getByText('Not stocked? Note a request').waitFor()
    await shot('requests-nomatch')
    await page.getByText('Not stocked? Note a request').click()
    await page.getByRole('textbox', { name: 'What did they ask for?' }).fill('umbrella')
    await page.getByText('Asked before').waitFor()
    return
  }
  if (scene === 'alternatives') {
    // The whole group shows for one member's name; the out-of-stock one gets a swap chip in the cart.
    await s.fill('Kingston')
    await page.getByText('Generic USB 32GB').first().waitFor()
    await page.screenshot({ path: `shots/${gate}/alternatives-search-${await page.evaluate(() => document.documentElement.dataset.palette ?? '')}-${await page.evaluate(() => localStorage.getItem('ks-mode'))}-${page.viewportSize()!.width}.png` })
    await page.getByRole('button', { name: /Kingston DataTraveler/ }).first().click()
    await page.getByText(/swap for/).waitFor()
    await page.screenshot({ path: `shots/${gate}/alternatives-cart-${await page.evaluate(() => document.documentElement.dataset.palette ?? '')}-${await page.evaluate(() => localStorage.getItem('ks-mode'))}-${page.viewportSize()!.width}.png` })
    // Checkout: the found-at-till prompt offers the swaps first.
    const amount = page.getByLabel('Amount tendered')
    await amount.fill('2000')
    await amount.press('Enter')
    await amount.press('Enter')
    await page.getByText(/In stock instead/).waitFor()
    return
  }
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

for (const palette of (process.env.SHOTS_PALETTES ?? 'studio,night').split(',')) {
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
