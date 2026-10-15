import { expect, test, type Page } from './fixtures'

// Combos (plugin, built in for Playwright). Needs `make e2e-seed`. Every combo and
// product is this run's own (tagged), so other specs never see a hint.
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const TEA = `CB Tea ${tag}` // Rs 300
const BUN = `CB Bun ${tag}` // Rs 100
const SOAP = `CB Soap ${tag}` // Rs 95
const OLD = `CB Old ${tag}` // disabled later
const PAIR = `Tea time ${tag}` // Tea + Bun at Rs 360 (made in the UI)
const BOGO = `Soap 3+1 ${tag}` // Soap ×4, Rs 95 off
const base = '/api/v1/x/combos'
const re = (s: string) => new RegExp(s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
const ids = { tea: 0, bun: 0, soap: 0, old: 0 }
const codes = { tea: `CBT${tag}`, bun: `CBB${tag}`, soap: `CBS${tag}`, old: `CBO${tag}` }

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

async function product(page: Page, name: string, price: string, barcode: string) {
  const cats = await json<{ id: number; name: string }[]>(page.request.get('/api/v1/admin/categories'))
  const res = await page.request.post('/api/v1/admin/products', { data: {
    name, nameLocal: '', barcode, categoryId: cats.find((c) => c.name === 'E2E Goods')!.id, unitId: 1, costPrice: '10', sellingPrice: price,
    wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } })
  expect(res.ok()).toBe(true)
  const id = (await res.json()).id as number
  expect((await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: id, newQuantity: '50', note: 'combos e2e', batchId: 0 } })).ok()).toBe(true)
  return id
}

async function combo(page: Page, name: string, discountType: string, discountValue: string, items: [number, string][], startsOn = '') {
  const res = await page.request.post(base, { data: { name, discountType, discountValue, startsOn, endsOn: '', items: items.map(([productId, qty]) => ({ productId, qty })) } })
  expect(res.ok()).toBe(true)
}

// Till helpers (till.spec.ts).
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
const search = (page: Page) => page.getByLabel('Scan a barcode or search products')
async function scan(page: Page, code: string) {
  await search(page).fill(code)
  await search(page).press('Enter')
}
const cart = (page: Page) => page.getByRole('region', { name: 'Cart' })
async function till(page: Page) {
  await login(page, '/till')
  await ensureOpen(page)
  const ns = page.getByRole('button', { name: 'New sale' })
  if (await cart(page).getByText(/Cart is empty/).isHidden()) {
    await ns.click()
    const yes = page.getByRole('dialog').getByRole('button', { name: /new sale|clear/i })
    if (await yes.isVisible().catch(() => false)) await yes.click()
  }
}

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage()
  await login(page, '/admin')
  ids.tea = await product(page, TEA, '300', codes.tea)
  ids.bun = await product(page, BUN, '100', codes.bun)
  ids.soap = await product(page, SOAP, '95', codes.soap)
  ids.old = await product(page, OLD, '50', codes.old)
  await combo(page, BOGO, 'fixed', '95', [[ids.soap, '4']])
  await page.close()
})

test('make a combo price deal in Apps → Combos', async ({ page }) => {
  await login(page, '/admin/x/combos')
  await page.getByRole('button', { name: 'New combo' }).click()
  const dlg = page.getByRole('dialog', { name: 'New combo' })
  await dlg.getByLabel('Name').fill(PAIR)
  for (const name of [TEA, BUN]) {
    await dlg.getByRole('button', { name: 'Add an item' }).click()
    await page.getByLabel('Find a product').fill(name)
    await page.getByLabel('Find a product').press('Enter')
  }
  await dlg.getByLabel('Combo price').fill('360')
  await expect(dlg.getByText(/saves Rs\. 40\.00/)).toBeVisible()
  await dlg.getByRole('button', { name: 'Save combo' }).click()
  await toast(page, /Combo added/)
  const card = page.getByRole('article').filter({ hasText: PAIR })
  await expect(card).toContainText('Rs. 360.00')
  await expect(card).toContainText('Not sold yet')
})

test('the till menu adds a combo as one group; + scales it; the receipt prints one block', async ({ page }) => {
  await till(page)
  await page.getByRole('button', { name: /Combos/ }).first().click()
  await page.getByRole('button', { name: new RegExp(PAIR) }).click()
  await page.getByRole('button', { name: /Add combo/ }).click()
  await toast(page, new RegExp(`${PAIR} added`))
  const label = `${PAIR} (combo)`
  await expect(cart(page).getByText(label)).toBeVisible()
  await cart(page).getByRole('button', { name: `More ${label}` }).click()
  await expect(cart(page).getByLabel(`Sets of ${label}`)).toHaveText('2')
  await expect(page.getByText(/Item discounts/)).toBeVisible()
  await page.getByRole('button', { name: /Complete sale/ }).click()
  const done = page.getByRole('dialog', { name: /complete/ })
  await expect(done).toContainText('Rs. 720.00')
  await expect(done).toContainText(label)
  await expect(done).toContainText(/2 x 400\.00 -> 360\.00/)
  await expect(done).toContainText(`- ${TEA} x2`)
  await page.keyboard.press('Enter')
})

test('scanned one by one: the hint applies a set, the line chip completes a part set', async ({ page }) => {
  await till(page)
  await scan(page, codes.tea)
  await scan(page, codes.bun)
  await page.getByRole('button', { name: 'Apply' }).click()
  await expect(cart(page).getByText(`${PAIR} (combo)`)).toBeVisible()
  await expect(cart(page).getByLabel(`Quantity of ${TEA}`)).toHaveCount(0) // no plain Tea line left
  for (let i = 0; i < 2; i++) await scan(page, codes.soap)
  await cart(page).getByRole('button', { name: re(`${BOGO} · add 2 × ${SOAP}`) }).click()
  await expect(cart(page).getByText(`${BOGO} (combo)`)).toBeVisible()
  await expect(cart(page).getByText(`${SOAP} × 4`)).toBeVisible()
})

test('removing one member asks, then the rest go back to full price', async ({ page }) => {
  await till(page)
  await scan(page, codes.tea)
  await scan(page, codes.bun)
  await page.getByRole('button', { name: 'Apply' }).click()
  await cart(page).getByRole('button', { name: `Remove ${BUN} from ${PAIR} (combo)` }).click()
  await page.getByRole('button', { name: 'Break it' }).click()
  await expect(cart(page).getByText(`${PAIR} (combo)`)).toHaveCount(0)
  await expect(cart(page).getByLabel(`Quantity of ${TEA}`)).toHaveValue('1')
  await expect(cart(page).getByText('Rs. 300.00').first()).toBeVisible()
  // What's left is part of the combo again.
  await expect(cart(page).getByRole('button', { name: re(`${PAIR} · add ${BUN}`) })).toBeVisible()
})

test('Settings → Combos auto apply: a full set rings as the combo by itself; a broken one stays apart till the cart empties', async ({ page }) => {
  await login(page, '/admin/settings')
  const sw = page.getByRole('switch', { name: /Apply combos automatically/ })
  await page.locator('label').filter({ hasText: 'Apply combos automatically' }).click()
  await expect(sw).toBeChecked()
  await toast(page, /Combos apply automatically/)
  try {
    await till(page)
    await scan(page, codes.tea)
    await expect(cart(page).getByRole('button', { name: re(`${PAIR} · add ${BUN}`) })).toBeVisible() // a part set still asks
    await scan(page, codes.bun)
    await expect(cart(page).getByText(`${PAIR} (combo)`)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Apply' })).toHaveCount(0)
    await cart(page).getByRole('button', { name: `Remove ${BUN} from ${PAIR} (combo)` }).click()
    await page.getByRole('button', { name: 'Break it' }).click()
    await scan(page, codes.bun)
    await expect(cart(page).getByLabel(`Quantity of ${BUN}`)).toHaveValue('1')
    await expect(cart(page).getByText(`${PAIR} (combo)`)).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Apply' })).toBeVisible() // still offered, by hand
    // Emptying the cart forgets the break: the next full set applies itself again.
    await cart(page).getByRole('button', { name: `Remove ${TEA}` }).click()
    await cart(page).getByRole('button', { name: `Remove ${BUN}` }).click()
    await expect(cart(page).getByText(/Cart is empty/)).toBeVisible()
    await scan(page, codes.tea)
    await scan(page, codes.bun)
    await expect(cart(page).getByText(`${PAIR} (combo)`)).toBeVisible()
  } finally {
    expect((await page.request.put(`${base}/settings`, { data: { autoApply: false } })).ok()).toBe(true)
  }
})

test('a disabled member blocks the combo; a future combo is not on the till', async ({ page }) => {
  await login(page, '/admin')
  await combo(page, `Old pair ${tag}`, 'percent', '10', [[ids.old, '1'], [ids.bun, '1']])
  await combo(page, `Later ${tag}`, 'percent', '10', [[ids.tea, '2']], '2099-01-01')
  expect((await page.request.delete(`/api/v1/admin/products/${ids.old}`)).ok()).toBe(true) // disables it
  await page.goto('/admin/x/combos')
  await expect(page.getByRole('article').filter({ hasText: `Old pair ${tag}` })).toContainText(`${OLD} is disabled`)
  await expect(page.getByRole('article').filter({ hasText: `Later ${tag}` })).toContainText(/Starts/)
  await till(page)
  await page.getByRole('button', { name: /Combos/ }).first().click()
  await expect(page.getByRole('button', { name: new RegExp(`Later ${tag}`) })).toHaveCount(0)
  await page.getByRole('button', { name: new RegExp(`Old pair ${tag}`) }).click()
  await expect(page.getByRole('button', { name: /Add combo/ })).toBeDisabled()
})

test('the Combos report counts the sale; Apps → Combos fits 360px', async ({ page }) => {
  await login(page, '/admin/x/combos/report')
  await page.getByRole('button', { name: 'Table' }).click()
  await expect(page.getByRole('row').filter({ hasText: PAIR })).toContainText('Rs. 80.00')
  await page.setViewportSize({ width: 360, height: 780 })
  await page.goto('/admin/x/combos')
  await expect(page.getByRole('article').filter({ hasText: PAIR })).toContainText(/Sold 2×/)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})
