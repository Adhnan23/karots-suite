import { expect, test, type Page } from './fixtures'

// Alternatives (plugin, built in for Playwright). Needs `make e2e-seed`. Every group
// and product is this run's own (tagged), so other specs never see a swap chip.
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const OUT = `AL Kingston ${tag}` // stock 0, Rs 1750
const SAN = `AL SanDisk ${tag}` // stock 14, Rs 1800 — Genuine
const GEN = `AL Generic ${tag}` // stock 25, Rs 900 — Cheap
const STRAY = `AL Stray ${tag}` // starts in another group
const GROUP = `AL USB ${tag}`
const OTHER = `AL Other ${tag}`
const base = '/api/v1/x/alternatives'
const ids = { out: 0, san: 0, gen: 0, stray: 0, group: 0 }

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

async function product(page: Page, name: string, price: string, stock: string) {
  const cats = await json<{ id: number; name: string }[]>(page.request.get('/api/v1/admin/categories'))
  const res = await page.request.post('/api/v1/admin/products', { data: {
    name, nameLocal: '', barcode: '', categoryId: cats.find((c) => c.name === 'E2E Goods')!.id, unitId: 1, costPrice: '10', sellingPrice: price,
    wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } })
  expect(res.ok()).toBe(true)
  const id = (await res.json()).id as number
  expect((await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: id, newQuantity: stock, note: 'alternatives e2e', batchId: 0 } })).ok()).toBe(true)
  return id
}

type Group = { id: number; name: string; tiers: { id: number; name: string; members: { productId: number }[] }[] }

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
async function addOut(page: Page) {
  await search(page).fill(OUT)
  await page.getByRole('button', { name: new RegExp(OUT) }).first().click()
  await expect(cart(page).getByText(OUT)).toBeVisible()
}

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage()
  await login(page, '/admin')
  ids.out = await product(page, OUT, '1750', '0')
  ids.san = await product(page, SAN, '1800', '14')
  ids.gen = await product(page, GEN, '900', '25')
  ids.stray = await product(page, STRAY, '500', '3')
  const other = await json<Group>(page.request.post(base, { data: { name: OTHER } }))
  expect((await page.request.post(`${base}/tiers/${other.tiers[0].id}/members`, { data: { productId: ids.stray } })).ok()).toBe(true)
  await page.close()
})

test('make a group, name its first tier while adding a second, and move a product in from another group', async ({ page }) => {
  await login(page, '/admin/x/alternatives')
  await page.getByRole('button', { name: 'New group' }).click()
  await page.getByRole('dialog', { name: 'New group' }).getByLabel('Name').fill(GROUP)
  await page.getByRole('button', { name: 'Add group' }).click()
  await expect(page.getByRole('heading', { name: GROUP })).toBeVisible()
  ids.group = Number(page.url().split('/').pop())
  const add = async (section: string, name: string) => {
    await page.getByRole('region', { name: section }).getByRole('button', { name: 'Add products' }).click()
    await page.getByLabel('Find a product').fill(name)
    await page.getByLabel('Find a product').press('Enter')
    await toast(page, new RegExp(`${name} added`))
  }
  await add('Products', OUT)
  await add('Products', SAN)
  await page.getByLabel('Reorder level').fill('10')
  await page.getByRole('button', { name: 'Save' }).click()
  await toast(page, /Saved/)

  await page.getByRole('button', { name: 'Add a quality tier' }).click()
  const dlg = page.getByRole('dialog', { name: 'Add a quality tier' })
  await dlg.getByLabel('The products already here are').fill('Genuine')
  await dlg.getByLabel('New tier').fill('Cheap')
  await dlg.getByRole('button', { name: 'Add tier' }).click()
  await toast(page, /Tier added/)
  await expect(page.getByRole('region', { name: 'Genuine' })).toContainText(SAN)
  await add('Cheap', GEN)

  // A product in another group: the picker says where, and adding asks first.
  await page.getByRole('region', { name: 'Cheap' }).getByRole('button', { name: 'Add products' }).click()
  await page.getByLabel('Find a product').fill(STRAY)
  await expect(page.getByText(`In ${OTHER}`)).toBeVisible()
  await page.getByLabel('Find a product').press('Enter')
  await page.getByRole('button', { name: 'Move here' }).click()
  await toast(page, new RegExp(`${STRAY} moved here`))
  await expect(page.getByRole('region', { name: 'Cheap' })).toContainText(STRAY)

  // The group list shows each tier against its level (Cheap is untracked).
  await page.getByRole('link', { name: /All groups/ }).click()
  const card = page.getByRole('link').filter({ hasText: GROUP })
  await expect(card).toContainText('Genuine')
  await expect(card).toContainText('reorder at 10')
  await expect(card).toContainText('not tracked')
})

test('the till finds the whole group with ⇄ pins and swaps an out-of-stock line', async ({ page }) => {
  await till(page)
  await search(page).fill(OUT)
  await expect(page.getByRole('button', { name: new RegExp(GEN) }).first()).toContainText('⇄ Cheap')
  await expect(page.getByRole('button', { name: new RegExp(SAN) }).first()).toContainText('⇄ Genuine')
  await page.getByRole('button', { name: new RegExp(OUT) }).first().click()
  const line = cart(page)
  await expect(line.getByText(/Out of stock — swap for/)).toBeVisible()
  // Best tier first: SanDisk (Genuine) before Generic (Cheap).
  const chips = line.getByRole('button', { name: /^⇄/ })
  await expect(chips.first()).toContainText(SAN)
  await chips.first().click()
  await toast(page, new RegExp(`Swapped to ${SAN}`))
  await expect(line.getByText(SAN)).toBeVisible()
  await expect(line.getByText(OUT)).toBeHidden()
  await expect(line.getByText(/swap for/)).toBeHidden()
})

test('checkout: the found-at-till prompt offers the swap before correcting the count', async ({ page }) => {
  await till(page)
  await addOut(page)
  const amount = page.getByLabel('Amount tendered')
  await amount.fill('2000')
  await amount.press('Enter')
  await amount.press('Enter')
  const prompt = page.getByRole('dialog', { name: 'Sell more than the count shows?' })
  await expect(prompt.getByText(/In stock instead/)).toBeVisible()
  await prompt.getByRole('button', { name: new RegExp(GEN) }).click()
  await expect(prompt).toBeHidden()
  await expect(cart(page).getByText(GEN)).toBeVisible()
  await expect(cart(page).getByText(OUT)).toBeHidden()
  // Nothing was sold and the count wasn't touched.
  const p = await json<{ stockQty: string }>(page.request.get(`/api/v1/till/products/${ids.out}`))
  expect(Number(p.stockQty)).toBe(0)
})

test('ⓘ lists the alternatives; Reorder hides covered items and pulls in a low tier', async ({ page }) => {
  await login(page, '/admin')
  const detail = await json<{ rows: { label: string; value: string }[] }>(page.request.get(`/api/v1/till/products/${ids.out}/detail`))
  expect(detail.rows).toContainEqual({ label: 'Alternative · Genuine', value: `${SAN} — 14 left · Rs. 1,800.00` })
  const g = await json<Group>(page.request.get(`${base}/${ids.group}`))
  const cheap = g.tiers.find((t) => t.name === 'Cheap')!
  expect((await page.request.put(`${base}/tiers/${cheap.id}`, { data: { name: 'Cheap', reorderLevel: 100 } })).ok()).toBe(true)
  type Row = { productId: number; covered: boolean; note: string }
  const list = async (qs: string) => json<{ items: Row[]; annotated: boolean; hidden: number }>(page.request.get(`/api/v1/admin/reorder?all=1&q=AL%20&${qs}`))
  const plain = await list('')
  expect(plain.annotated).toBe(true)
  const out = plain.items.find((r) => r.productId === ids.out)!
  expect(out.covered).toBe(true) // Genuine still has 14 > 10
  expect(out.note).toContain('Covered')
  const hid = await list('hideCovered=1')
  const got = hid.items.map((r) => r.productId)
  expect(got).not.toContain(ids.out)
  expect(got).toContain(ids.gen) // not low on its own; its tier (28 ≤ 100) is
  expect(hid.items.find((r) => r.productId === ids.gen)!.note).toContain('is low')

  await page.goto('/admin/reorder')
  await page.getByLabel('Hide covered items').check()
  await page.getByLabel('Search products').fill(`AL Generic ${tag}`)
  await expect(page.getByText(/is low/).first()).toBeVisible()
})

test('the product form moves a product out of its group', async ({ page }) => {
  await login(page, `/admin/products?q=${encodeURIComponent(GEN)}`)
  await page.getByText(GEN).filter({ visible: true }).first().click()
  const sel = page.getByLabel('Alternatives group')
  await expect(sel).toHaveValue(String(ids.group))
  await expect(page.getByLabel('Alternatives tier')).toBeVisible()
  await sel.selectOption({ label: 'None' })
  await page.getByRole('button', { name: 'Save' }).click()
  await toast(page, /saved|updated/i)
  expect((await json<{ tierId: number }>(page.request.get(`${base}/product/${ids.gen}`))).tierId).toBe(0)
})

test('Apps → Alternatives fits 360px', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 })
  for (const path of ['/admin/x/alternatives', `/admin/x/alternatives/${ids.group}`]) {
    await login(page, path)
    await expect(page.getByText(GROUP).first()).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360)
  }
})
