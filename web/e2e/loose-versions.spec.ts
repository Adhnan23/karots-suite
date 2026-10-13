import { expect, test, type Page } from './fixtures'

// Loose versions (M7.6d): a 25 kg bag gets its by-the-kg twin in one sheet, a run takes
// what was really weighed out, and an alternatives swap to it asks how many. Needs
// `make e2e-seed`; Product Plus and Alternatives are built in for Playwright. Every
// product, field and group is this run's own (tagged).
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const BAG = `LV Rice bag ${tag}`
const LOOSE = `${BAG} (loose)`
const OTHER = `LV Rice other ${tag}` // a second bag in the group, unticked check
const FIELD = `LV Grain ${tag}`
const ids = { bag: 0, loose: 0, kg: 0, group: 0 }
const expiry = new Date(Date.now() + 20 * 86_400_000).toISOString().slice(0, 10)

async function login(page: Page, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}
const json = async <T,>(p: Promise<{ json: () => Promise<unknown> }>) => (await (await p).json()) as T
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()

async function bag(page: Page, name: string, qty: string) {
  const cats = await json<{ id: number; name: string }[]>(page.request.get('/api/v1/admin/categories'))
  const res = await page.request.post('/api/v1/admin/products', { data: {
    name, nameLocal: '', barcode: '', categoryId: cats.find((c) => c.name === 'E2E Goods')!.id, unitId: 1, costPrice: '6500', sellingPrice: '7200',
    wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } })
  expect(res.ok()).toBe(true)
  const id = (await res.json()).id as number
  expect((await page.request.post('/api/v1/admin/inventory/intake/restock', { data: {
    productId: id, quantity: qty, costPrice: '6500', sellingPrice: '', wholesalePrice: '', scanned: '', batchId: 0, expiry,
  } })).ok()).toBe(true)
  return id
}

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage()
  await login(page, '/admin')
  ids.bag = await bag(page, BAG, '3')
  const other = await bag(page, OTHER, '2')
  ids.kg = (await json<{ id: number; abbreviation: string }[]>(page.request.get('/api/v1/admin/units'))).find((u) => u.abbreviation === 'kg')!.id
  // A Product Plus value on the bag, to be copied.
  const pp = '/api/v1/x/productplus'
  const field = await json<{ id: number }>(page.request.post(`${pp}/fields`, { data: {
    label: FIELD, type: 'text', defaultValue: '', hint: '', required: false, searchable: false, showAtTill: false, printOnLabel: false,
    showInIntake: false, options: [], scope: [],
  } }))
  expect((await page.request.put(`${pp}/values`, { data: [{ productId: ids.bag, fieldId: field.id, value: 'Samba' }] })).ok()).toBe(true)
  // Both bags in one alternatives group.
  const g = await json<{ id: number; tiers: { id: number }[] }>(page.request.post('/api/v1/x/alternatives', { data: { name: `LV Rice ${tag}` } }))
  ids.group = g.id
  for (const productId of [ids.bag, other]) {
    expect((await page.request.post(`/api/v1/x/alternatives/tiers/${g.tiers[0].id}/members`, { data: { productId } })).ok()).toBe(true)
  }
  await page.close()
})

test('make a loose version with Convert now 1: 25 kg with the bag’s expiry, Product Plus copied, not an alternative unless ticked', async ({ page }) => {
  await login(page, `/admin/products?q=${encodeURIComponent(BAG)}`)
  await page.getByText(BAG, { exact: true }).locator('visible=true').first().click()
  await page.getByRole('button', { name: /Make a loose version/ }).click()
  const d = page.getByRole('dialog', { name: `Sell ${BAG} loose` })
  await expect(d.getByLabel('Name')).toHaveValue(LOOSE)
  await d.getByLabel('Loose unit').selectOption(String(ids.kg))
  await d.getByLabel('Ratio').fill('25')
  await expect(d.getByText('Costs about Rs. 260.00 a kg')).toBeVisible()
  await d.getByLabel('Loose selling price').fill('300')
  await d.getByLabel('Convert now').fill('1')
  await expect(d.getByText('→ 25 kg in stock')).toBeVisible()
  await expect(d.getByLabel(FIELD)).toHaveValue('Samba')
  await expect(d.getByText(/Also offer as an alternative in LV Rice/)).toBeVisible() // offered, left off
  await d.getByRole('button', { name: 'Make loose version' }).click()
  await toast(page, new RegExp(`${LOOSE.replace(/[()]/g, '\\$&')} made — 25 kg in stock`))
  // The new product's form opens.
  await expect(page.getByRole('dialog', { name: `Edit ${LOOSE}` })).toBeVisible()

  const list = await json<{ items: { id: number; name: string }[] }>(page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(LOOSE)}`))
  ids.loose = list.items.find((p) => p.name === LOOSE)!.id
  const lots = await json<{ stockQty: string; lots: { expiryDate: string; qtyRemaining: string; costPrice: string }[] }>(page.request.get(`/api/v1/admin/stock/lots/${ids.loose}`))
  expect(Number(lots.stockQty)).toBe(25)
  expect(lots.lots[0].expiryDate).toBe(expiry)
  expect(Number(lots.lots[0].costPrice)).toBe(260)
  const bagLots = await json<{ stockQty: string }>(page.request.get(`/api/v1/admin/stock/lots/${ids.bag}`))
  expect(Number(bagLots.stockQty)).toBe(2)
  const form = await json<{ label: string; value: string }[]>(page.request.get(`/api/v1/x/productplus/form?productId=${ids.loose}&categoryId=0&where=form`))
  expect(form.find((f) => f.label === FIELD)?.value).toBe('Samba')
  const member = await json<{ tierId: number }>(page.request.get(`/api/v1/x/alternatives/product/${ids.loose}`))
  expect(member.tierId).toBe(0)
})

test('a run that weighs out 24.6 kg says so and puts the cost on the 24.6', async ({ page }) => {
  await login(page, '/admin/conversions')
  const row = page.getByRole('row').or(page.getByRole('listitem')).filter({ hasText: LOOSE }).locator('visible=true').first()
  await row.getByRole('button', { name: 'Run' }).click()
  await expect(page.getByLabel('Got')).toHaveValue('25')
  await page.getByLabel('Got').fill('24.6')
  await expect(page.getByText('0.4 kg less than 25 — cost per kg Rs. 4.23 higher')).toBeVisible()
  await page.getByRole('button', { name: 'Make 24.6 kg' }).click()
  await toast(page, /→ 24\.6 kg/)
  const lots = await json<{ stockQty: string; lots: { qtyRemaining: string; costPrice: string }[] }>(page.request.get(`/api/v1/admin/stock/lots/${ids.loose}`))
  expect(Number(lots.stockQty)).toBe(49.6)
  expect(lots.lots.map((l) => Number(l.costPrice))).toContain(264.23) // 6500 / 24.6
})

test('a till swap to an alternative sold by the kg asks how many; Cancel leaves the line', async ({ page }) => {
  // Tick the loose version into the group now (the loose sheet left it out).
  await login(page, '/admin')
  const g = await json<{ tiers: { id: number }[] }>(page.request.get(`/api/v1/x/alternatives/${ids.group}`))
  expect((await page.request.post(`/api/v1/x/alternatives/tiers/${g.tiers[0].id}/members`, { data: { productId: ids.loose } })).ok()).toBe(true)

  await page.goto('/till')
  const open = page.getByRole('dialog', { name: 'Open the till' })
  if (await open.isVisible()) {
    await open.getByLabel(/Count of .*500\.00/).fill('2')
    await open.getByRole('button', { name: 'Open the till' }).click()
    const skip = page.getByRole('button', { name: 'Skip' })
    await skip.waitFor({ state: 'visible', timeout: 1500 }).then(() => skip.click()).catch(() => undefined)
  }
  const cart = page.getByRole('region', { name: 'Cart' })
  if (await cart.getByText(/Cart is empty/).isHidden()) {
    await page.getByRole('button', { name: 'New sale' }).click()
    const yes = page.getByRole('dialog').getByRole('button', { name: /new sale|clear/i })
    if (await yes.isVisible().catch(() => false)) await yes.click()
  }
  await page.getByLabel('Scan a barcode or search products').fill(BAG)
  await page.getByRole('button', { name: new RegExp(`^${BAG}(?! \\()`) }).first().click()
  for (let i = 0; i < 2; i++) await cart.getByRole('button', { name: `More ${BAG}` }).click()
  const chip = cart.getByRole('button', { name: new RegExp(`⇄ ${LOOSE.replace(/[()]/g, '\\$&')} · 49\\.6 kg left`) })
  await chip.click()
  const ask = page.getByRole('dialog', { name: `How many kg of ${LOOSE}?` })
  await ask.getByRole('button', { name: 'Cancel' }).click()
  await expect(ask).toBeHidden()
  await expect(cart.getByText(BAG, { exact: true })).toBeVisible()

  await chip.click()
  await ask.getByRole('textbox', { name: 'How many kg' }).fill('2.5')
  await ask.getByRole('button', { name: 'Swap' }).click()
  await expect(cart.getByText(LOOSE, { exact: true })).toBeVisible()
  await expect(cart.getByText(BAG, { exact: true })).toHaveCount(0)
  await expect(cart.getByText('Rs. 750.00', { exact: true })).toBeVisible() // 2.5 kg × 300
  await page.getByRole('button', { name: 'New sale' }).click()
  const yes = page.getByRole('dialog').getByRole('button', { name: /new sale|clear/i })
  if (await yes.isVisible().catch(() => false)) await yes.click()
})
