import type { APIResponse } from '@playwright/test'
import { expect, freshProduct, test, type Page } from './fixtures'

// Packs (M7.7a): one product counted in tablets, sold by the card or loose — full
// cards ring at the card price, a card's code scans a card, a card size nobody set can
// be set at the till (and waits on Items to review), deliveries come in by the card,
// and an old card + "(loose)" twin folds into one product. Needs `make e2e-seed`.
// Serial: one drawer.
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const PANADOL = `PK Panadol ${tag}`
const THYROX = `PK Thyrox ${tag}`
const CARD = `PK Amoxil card ${tag}`
const LOOSE = `PK Amoxil ${tag} (loose)`
const CODE = `479${tag}0013`
const ids = { panadol: 0, thyrox: 0, card: 0, loose: 0 }

async function login(page: Page, phone: string, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}
async function till(page: Page) {
  await login(page, '0770000002', '/till')
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) {
    await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
    await page.reload()
  }
}
const ok = async (p: Promise<APIResponse>) => {
  const r = await p
  if (!r.ok()) throw new Error(await r.text())
  return r
}
async function priced(page: Page, name: string, cost: string, sell: string, qty: string, unitId = 1) {
  const id = await freshProduct(page, name)
  const p = await (await page.request.get(`/api/v1/admin/products/${id}`)).json()
  await ok(page.request.put(`/api/v1/admin/products/${id}`, { data: { name, nameLocal: '', barcode: '', categoryId: p.categoryId, unitId,
    costPrice: cost, sellingPrice: sell, wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0,
    repriceStock: false, dupSourceName: '' } }))
  await ok(page.request.post('/api/v1/admin/inventory/intake/restock', { data: {
    productId: id, quantity: qty, costPrice: '', sellingPrice: '', wholesalePrice: '', scanned: '', batchId: 0, expiry: '2027-06-30' } }))
  return id
}
const search = async (page: Page, text: string) => {
  const s = page.getByLabel('Scan a barcode or search products')
  await s.fill(text)
  await s.press('Enter')
}
const line = (page: Page, name: string) => page.getByLabel(`Quantity of ${name}`)

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage()
  await login(page, '0770000001', '/admin')
  ids.panadol = await priced(page, PANADOL, '18', '26', '100')
  await ok(page.request.post(`/api/v1/admin/products/${ids.panadol}/packs`, { data: { name: 'Card', size: '12', sellingPrice: '250', wholesalePrice: '', barcode: CODE } }))
  ids.thyrox = await priced(page, THYROX, '9', '12', '100')
  const pkt = (await (await page.request.get('/api/v1/admin/units')).json()).find((u: { abbreviation: string }) => u.abbreviation === 'pkt').id
  ids.card = await priced(page, CARD, '300', '420', '3', pkt)
  const r = await ok(page.request.post(`/api/v1/admin/products/${ids.card}/loose`, { data: {
    name: LOOSE, unitId: 1, ratio: '10', sellingPrice: '45', barcode: '', convertNow: '', extra: {} } }))
  ids.loose = (await r.json()).productId
  await page.close()
})

test('a product sold by the card: chooser, 14 = 1 card + 2, the card code adds a card, the receipt shows the split', async ({ page }) => {
  await till(page)
  await search(page, PANADOL)
  const pick = page.getByRole('dialog', { name: PANADOL })
  await expect(pick.getByRole('button', { name: /1 Card ×12/ })).toContainText('Rs. 250.00')
  await page.keyboard.press('2')
  await expect(line(page, PANADOL)).toHaveValue('12')
  await line(page, PANADOL).fill('14')
  await line(page, PANADOL).press('Enter')
  await expect(page.getByText('= 1 Card + 2')).toBeVisible()
  await expect(page.getByText('Rs. 302.00').first()).toBeVisible()
  await search(page, CODE) // the card's own barcode: no chooser, a whole card
  await expect(line(page, PANADOL)).toHaveValue('26')
  await expect(page.getByText('= 2 Cards + 2')).toBeVisible()
  await page.getByRole('button', { name: /Complete sale/ }).click()
  const done = page.getByRole('dialog', { name: /complete/ })
  await expect(done).toContainText('Rs. 552.00')
  await expect(done).toContainText('(2 Cards + 2)')
  await page.keyboard.press('Enter')
})

test('no card size yet: Set pack asks once, sells 2 cards, and the size waits on Items to review', async ({ page }) => {
  await till(page)
  await search(page, THYROX)
  await page.getByRole('button', { name: 'Set pack' }).click()
  await page.getByLabel('Units in one pack').fill('28')
  await page.getByLabel('Packs to sell').fill('2')
  await page.getByRole('button', { name: 'Save & add' }).click()
  await expect(line(page, THYROX)).toHaveValue('56')
  await expect(page.getByText('= 2 Cards')).toBeVisible()
  await page.getByRole('button', { name: 'New sale' }).click()
  await page.getByRole('button', { name: 'Clear sale' }).click().catch(() => undefined)

  await login(page, '0770000001', '/admin/products/review')
  await expect(page.getByRole('button', { name: new RegExp(THYROX) }).getByText('Card ×28 set at the till — check the size').first()).toBeVisible()
})

test('a delivery by the card: 3 cards at 250 owe 750 and bring in 36 tablets', async ({ page }) => {
  await login(page, '0770000001', '/admin/purchases')
  const sup = `PK Supplier ${tag}`
  await page.request.post('/api/v1/admin/suppliers', { data: { name: sup, contact: '', phone: '', address: '', creditDays: 0, openingBalance: '' } })
  await page.getByRole('link', { name: 'Receive delivery' }).click()
  const sel = page.getByLabel('Supplier')
  await sel.selectOption((await sel.locator('option', { hasText: sup }).first().getAttribute('value'))!)
  await page.getByLabel('Add a product').fill(PANADOL)
  await page.getByRole('button', { name: new RegExp(`^${PANADOL}`) }).click()
  await page.getByRole('group', { name: `Count ${PANADOL} in` }).getByRole('button', { name: 'Card ×12' }).click()
  await page.getByLabel(`Quantity of ${PANADOL}`).fill('3')
  await page.getByLabel(`Cost of ${PANADOL}`).fill('250')
  await expect(page.getByText('Rs. 750.00').first()).toBeVisible()
  const before = Number((await (await page.request.get(`/api/v1/admin/products/${ids.panadol}`)).json()).stockQty)
  await page.keyboard.press('F10')
  await expect(page.locator('[data-sonner-toast]').first()).toBeVisible()
  await expect.poll(async () => Number((await (await page.request.get(`/api/v1/admin/products/${ids.panadol}`)).json()).stockQty)).toBe(before + 36)
})

test('fold an old card + loose twin: one product, the card a pack of it', async ({ page }) => {
  await login(page, '0770000001', `/admin/products?q=${encodeURIComponent(LOOSE)}`)
  await page.getByText(LOOSE, { exact: true }).locator('visible=true').first().click()
  await page.getByRole('button', { name: new RegExp(`Fold ${CARD} into packs`) }).click()
  await expect(page.getByLabel('Product name after folding')).toHaveValue(`PK Amoxil ${tag}`)
  await page.getByRole('button', { name: 'Fold into one product' }).click()
  await expect(page.locator('[data-sonner-toast]').filter({ hasText: /is now a Card ×10/ }).first()).toBeVisible()
  const p = await (await page.request.get(`/api/v1/admin/products/${ids.loose}`)).json()
  expect(p.name).toBe(`PK Amoxil ${tag}`)
  expect(Number(p.stockQty)).toBe(30)
  const packs = await (await page.request.get(`/api/v1/admin/products/${ids.loose}/packs`)).json()
  expect(packs).toMatchObject([{ name: 'Card', size: '10', sellingPrice: '420.00' }])
})
