import { expect, test, type Page } from './fixtures'

// Print & Copy (plugin, built in for Playwright). Needs `make e2e-seed`. The service,
// its paper and group are this run's own (tagged).
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const COPY = `PC Copy ${tag}`
const PAPER = `PC Paper ${tag}`
const CARD = `PC Card ${tag}`
const LINE = `${COPY} A4 · B&W · 1-side`

async function login(page: Page, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}

const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()
const json = async <T,>(p: Promise<{ json: () => Promise<unknown> }>) => (await (await p).json()) as T

async function ensureOpen(page: Page) {
  const dlg = page.getByRole('dialog', { name: 'Open the till' })
  if (await dlg.isVisible()) {
    await dlg.getByLabel(/Count of .*500\.00/).fill('2')
    await dlg.getByRole('button', { name: 'Open the till' }).click()
    await expect(dlg).toBeHidden()
    const skip = page.getByRole('button', { name: 'Skip' })
    await skip.waitFor({ state: 'visible', timeout: 1500 }).then(() => skip.click()).catch(() => undefined)
  }
}
const cart = (page: Page) => page.getByRole('region', { name: 'Cart' })
async function till(page: Page) {
  await login(page, '/till')
  await ensureOpen(page)
  if (await cart(page).getByText(/Cart is empty/).isHidden()) {
    await page.getByRole('button', { name: 'New sale' }).click()
    const yes = page.getByRole('dialog').getByRole('button', { name: /new sale|clear/i })
    if (await yes.isVisible().catch(() => false)) await yes.click()
  }
}
async function openService(page: Page) {
  await page.getByRole('toolbar', { name: 'Groups' }).getByRole('button', { name: /Print & Copy/ }).click()
  await page.getByRole('button', { name: new RegExp(COPY) }).first().click()
}
const paperLeft = async (page: Page) => {
  const p = await json<{ items: { name: string; stockQty: string }[] }>(page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(PAPER)}`))
  return Number(p.items.find((x) => x.name === PAPER)!.stockQty)
}

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage()
  await login(page, '/admin')
  const cats = await json<{ id: number; name: string }[]>(page.request.get('/api/v1/admin/categories'))
  const res = await page.request.post('/api/v1/admin/products', { data: {
    name: PAPER, nameLocal: '', barcode: `PCP${tag}`, categoryId: cats.find((c) => c.name === 'E2E Goods')!.id, unitId: 1, costPrice: '2', sellingPrice: '0',
    wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } })
  expect(res.ok()).toBe(true)
  const id = (await res.json()).id as number
  expect((await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: id, newQuantity: '500', note: 'print e2e', batchId: 0 } })).ok()).toBe(true)
  await page.close()
})

test('make a print service with a bulk band and its paper', async ({ page }) => {
  await login(page, '/admin/x/documents')
  await page.getByRole('button', { name: 'New print service' }).click()
  const dlg = page.getByRole('dialog', { name: 'New print service' })
  if (await dlg.getByLabel('Start from').isVisible()) await dlg.getByLabel('Start from').selectOption('0')
  await dlg.getByLabel('Name').fill(COPY)
  await dlg.getByRole('button', { name: /Category|Pick a category/ }).first().click()
  await page.getByRole('option', { name: /E2E Goods/ }).or(page.getByRole('button', { name: /E2E Goods/ })).first().click()
  await dlg.getByRole('button', { name: 'Add' }).click()
  await toast(page, /now set its prices and paper/)

  const grid = page.getByRole('dialog', { name: `Prices & paper — ${COPY}` })
  const a4 = grid.getByRole('region', { name: 'A4' })
  await a4.getByLabel('A4 B&W 1-side from 1').fill('10')
  await a4.getByLabel('A4 B&W 2-side from 1').fill('15')
  await a4.getByRole('button', { name: 'Bulk price band' }).click()
  await a4.getByLabel('A4 B&W 1-side from 100').fill('7')
  await a4.getByRole('button', { name: 'Pick the paper' }).click()
  await page.getByLabel('Find a product').fill(PAPER)
  await page.getByLabel('Find a product').press('Enter')
  await expect(a4.getByText(PAPER)).toBeVisible()
  await grid.getByRole('button', { name: 'Save' }).click()
  await toast(page, new RegExp(`${COPY} saved`))
  const card = page.getByRole('article').filter({ hasText: COPY })
  await expect(card).toContainText('3 prices')
  await expect(card).toContainText('1 paper')
})

test('the till prices a job from its band, adds a fixed-qty line that keeps a discount, and records the job', async ({ page }) => {
  await till(page)
  await openService(page)
  await page.getByLabel('Number of copies').fill('120')
  await expect(page.getByLabel('Price bands').getByText('100+ · Rs. 7.00')).toBeVisible()
  await expect(page.getByText('Rs. 840.00').first()).toBeVisible()
  await page.getByLabel('Number of copies').press('Enter')
  await toast(page, /×120 added/)
  await expect(cart(page).getByText(LINE)).toBeVisible()
  await expect(cart(page).getByRole('button', { name: `More ${LINE}` })).toHaveCount(0) // qty is fixed
  await cart(page).getByRole('button', { name: /Discount/ }).click()
  await page.getByLabel(`Discount for ${LINE}`).fill('1')
  await page.getByLabel(`Discount for ${LINE}`).press('Enter')
  const before = await paperLeft(page)
  await page.getByRole('button', { name: /Complete sale/ }).click()
  const done = page.getByRole('dialog', { name: /complete/ })
  await expect(done).toContainText('Rs. 720.00')
  await page.keyboard.press('Enter')
  expect(await paperLeft(page)).toBe(before - 120)
  await expect.poll(async () => {
    const r = await json<{ rows: { name: string; copies: string }[] }>(page.request.get('/api/v1/x/documents/report?preset=today'))
    return r.rows.find((x) => x.name === COPY)?.copies
  }).toBe('120')
})

test('picking the service from a group card opens its job panel', async ({ page }) => {
  await login(page, '/admin')
  const svc = (await json<{ name: string; productId: number }[]>(page.request.get('/api/v1/x/documents/services'))).find((s) => s.name === COPY)!
  const g = await json<{ id: number }>(page.request.post('/api/v1/admin/groups', { data: { name: CARD, emoji: '', parentId: 0 } }))
  expect((await page.request.post(`/api/v1/admin/groups/${g.id}/items`, { data: { productId: svc.productId, emoji: '' } })).ok()).toBe(true)
  await till(page)
  await page.getByRole('toolbar', { name: 'Groups' }).getByRole('button', { name: CARD }).click()
  await page.getByRole('button', { name: new RegExp(COPY) }).first().click()
  await expect(page.getByRole('region', { name: COPY })).toBeVisible()
  await expect(page.getByLabel('Number of copies')).toBeVisible()
  await expect(cart(page).getByText(/Cart is empty/)).toBeVisible() // nothing added on the pick
})

test('typing the service at the till finds it and Enter opens its job panel', async ({ page }) => {
  await till(page)
  await page.getByLabel('Scan a barcode or search products').fill(COPY)
  await page.getByLabel('Scan a barcode or search products').press('Enter')
  await expect(page.getByRole('region', { name: COPY })).toBeVisible()
  await expect(cart(page).getByText(/Cart is empty/)).toBeVisible()
})

test('shop use takes the paper off stock with no sale', async ({ page }) => {
  await till(page)
  await openService(page)
  const before = await paperLeft(page)
  await page.getByRole('button', { name: '2-side' }).click()
  await page.getByLabel('Number of copies').fill('5')
  await page.getByRole('button', { name: 'Shop use' }).click()
  await page.getByRole('button', { name: 'Record 5 as shop use' }).click()
  await toast(page, /Shop use recorded/)
  expect(await paperLeft(page)).toBe(before - 3) // 5 double-sided = 3 sheets
  await expect(cart(page).getByText(/Cart is empty/)).toBeVisible()
})

test('a spoilt job comes back through Returns: refunded, nothing restocked', async ({ page }) => {
  await login(page, '/till/returns')
  await ensureOpen(page)
  const before = await paperLeft(page)
  await page.getByLabel('Find a sale').fill(COPY)
  await page.getByText(/^S-\d+/).first().click()
  await expect(page.getByText(/nothing goes back on a shelf/)).toBeVisible()
  await page.getByRole('button', { name: `More ${LINE}` }).click()
  await page.getByRole('button', { name: /Refund Rs\. 6\.00 cash/ }).click()
  await expect(page.getByRole('dialog', { name: 'Print the receipt?' })).toContainText(/CASH REFUND\s+Rs\. 6\.00/)
  expect(await paperLeft(page)).toBe(before)
  await expect.poll(async () => {
    const r = await json<{ rows: { name: string; copies: string }[] }>(page.request.get('/api/v1/x/documents/report?preset=today'))
    return r.rows.find((x) => x.name === COPY)?.copies
  }).toBe('119')
})

test('Print & Copy admin and report fit a phone', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 760 })
  for (const path of ['/admin/x/documents', '/admin/x/documents/report']) {
    await login(page, path)
    await expect(page.getByRole('heading', { name: 'Print & Copy' })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360)
  }
})
