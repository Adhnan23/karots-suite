import { expect, test, type Page } from './fixtures'

// The till's Low stock tab: its own permission; with Counter operations too,
// Order these opens the Suppliers tab's Order with the lines filled in.
test.setTimeout(90_000)

async function signIn(page: Page, phone: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}

test('a cashier sees Low stock only when ticked, and orders through Suppliers', async ({ page, browser }) => {
  await signIn(page, '0770000001')
  const stamp = String(Date.now()).slice(-7)
  const phone = `072${stamp}`
  const user = { name: `Low Stock ${stamp.slice(-4)}`, phone, role: 'cashier', pin: '1234', receiptPrinter: '', canHandleSuppliers: false, canManageCredit: false, canSeeCost: false, canSeeLowStock: false }
  const made = await page.request.post('/api/v1/admin/users', { data: user })
  expect(made.ok(), await made.text()).toBeTruthy()
  const userId = (await made.json()).id
  const sup = await (await page.request.post('/api/v1/admin/suppliers', { data: { name: `E2E Rep ${stamp}` } })).json()
  const cats = await (await page.request.get('/api/v1/admin/categories')).json()
  const cat = (cats.items ?? cats).find((c: { name: string }) => c.name === 'E2E Goods')
  const item = `E2E Notebook ${stamp}`
  const p = await page.request.post('/api/v1/admin/products', { data: { name: item, nameLocal: '', barcode: '', categoryId: cat.id, unitId: 1, costPrice: '120', sellingPrice: '160', wholesalePrice: '', taxRate: '', reorderLevel: 5, reorderAuto: false, trackSerial: false, warrantyMonths: 0, supplierId: sup.id, repriceStock: false, dupSourceName: '' } })
  expect(p.ok(), await p.text()).toBeTruthy()

  const ctx = await browser.newContext()
  const cashier = await ctx.newPage()
  await signIn(cashier, phone)
  await cashier.goto('/till/low-stock')
  await expect(cashier).toHaveURL(/\/till$/) // no tick → back to Sell
  expect((await cashier.request.get('/api/v1/till/low-stock')).status()).toBe(403)

  // Ticked: the tab lists it, with no cost (no See cost) and no way to order yet.
  const tick = async (extra: object) => {
    const r = await page.request.put(`/api/v1/admin/users/${userId}`, { data: { ...user, pin: '', canSeeLowStock: true, ...extra } })
    expect(r.ok(), await r.text()).toBeTruthy()
  }
  await tick({})
  await cashier.goto('/till/low-stock')
  const group = cashier.getByRole('region', { name: sup.name })
  await expect(group).toContainText(item)
  await expect(group).not.toContainText('cost')
  await expect(group.getByRole('button', { name: /Order these/ })).toHaveCount(0)

  // With Counter operations too: Order these → Suppliers, Order, lines filled in.
  await tick({ canHandleSuppliers: true })
  await cashier.reload()
  await group.getByRole('button', { name: /Order these/ }).click()
  await expect(cashier).toHaveURL(/\/till\/suppliers/)
  await expect(cashier.getByRole('heading', { name: sup.name })).toBeVisible()
  await expect(cashier.getByRole('list', { name: 'Lines' })).toContainText(item)
  await cashier.getByRole('button', { name: 'Place order · 1 item' }).click()
  await expect.poll(async () => (await (await cashier.request.get(`/api/v1/till/suppliers/${sup.id}`)).json()).orders.length).toBe(1)
  await ctx.close()
})
