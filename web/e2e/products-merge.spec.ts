import { expect, test, type Page } from './fixtures'

// Merge products (M7.6a). Needs `make e2e-seed`. Each run makes its own pair.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString().slice(-7)
const DUP_CODE = `770${tag}3`

async function signIn(page: Page, phone: string, path: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(path)
  await page.waitForLoadState('networkidle')
}

test('Items to review → Same as an existing one: the old code scans to the kept product at its old price', async ({ page }) => {
  await signIn(page, '0770000003', '/admin')
  const quick = async (name: string, price: string, barcode: string) =>
    (await (await page.request.post('/api/v1/till/quick-item', { data: { name, price, qty: '1', barcode, unitId: 0 } })).json()).id as number
  const keep = await quick(`Keep ${tag}`, '100', '')
  await page.request.post(`/api/v1/admin/products/${keep}/review-done`)
  const dup = await quick(`Dup ${tag}`, '90', DUP_CODE)
  await page.request.post('/api/v1/admin/inventory/intake/restock', { data: { productId: keep, quantity: '3', costPrice: '', sellingPrice: '', wholesalePrice: '', scanned: '' } })
  await page.request.post('/api/v1/admin/inventory/intake/restock', { data: { productId: dup, quantity: '2', costPrice: '', sellingPrice: '', wholesalePrice: '', scanned: '' } })

  await page.goto('/admin/products/review')
  await page.getByRole('listitem').or(page.getByRole('row')).filter({ hasText: `Dup ${tag}` }).getByRole('button', { name: 'Same as an existing one' }).click()
  await page.getByLabel('Find a product').fill(`Keep ${tag}`)
  await page.getByRole('button', { name: new RegExp(`^Keep ${tag}`) }).first().click()
  const dlg = page.getByRole('dialog', { name: `Merge into Keep ${tag}?` })
  await expect(dlg).toContainText('1 barcode')
  await dlg.getByText(/Keep Rs\. 90\.00 on those/).click()
  await dlg.getByRole('button', { name: 'Merge', exact: true }).click()
  await expect(page.locator('[data-sonner-toast]').filter({ hasText: `Dup ${tag} merged into Keep ${tag}` }).first()).toBeVisible()
  await expect(page.getByText(`Dup ${tag}`)).toHaveCount(0)

  const p = await (await page.request.get(`/api/v1/till/products/barcode/${DUP_CODE}`)).json()
  expect(p.id).toBe(keep)
  expect(Number(p.stockQty)).toBe(7) // 1 + 3 kept, 1 + 2 from the duplicate
  const lots = await (await page.request.get(`/api/v1/till/products/${keep}/lots`)).json()
  expect(JSON.stringify(lots)).toContain('90.00') // its stock still sells at its own price
})

test('a merged product is gone from Products, Show disabled included', async ({ page }) => {
  await signIn(page, '0770000003', '/admin/products')
  await page.getByLabel('Search products').fill(`Dup ${tag}`)
  await page.waitForLoadState('networkidle')
  const res = await (await page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(`Dup ${tag}`)}&disabled=1`)).json()
  expect(res.items).toHaveLength(0)
})
