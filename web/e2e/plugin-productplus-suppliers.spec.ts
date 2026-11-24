import { expect, test, type Page } from './fixtures'

// Product Plus → Supplier by field (PPF-10…12, SUP-12): pick a field, give one of
// its values a supplier, and that supplier's products list has the product.
// The supplier field is one per shop: this spec clears it when it's done.
const tag = Date.now().toString().slice(-6)
const BRAND = `Brand ${tag}`
const VALUE = `Bosch ${tag}`
const SUP = `Lanka Auto ${tag}`
const base = '/api/v1/x/productplus'

async function login(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}

const post = async (page: Page, url: string, data: unknown) => {
  const res = await page.request.post(url, { data })
  expect(res.ok()).toBe(true)
  return (await res.json()) as { id: number }
}

test('a value\'s supplier reaches products with no supplier of their own', async ({ page }) => {
  await login(page)
  const cat = await post(page, '/api/v1/admin/categories/quick', { name: `SBF Cat ${tag}`, parentId: 0 })
  const field = await post(page, `${base}/fields`, { label: BRAND, type: 'text', defaultValue: '', hint: '', required: false, searchable: false,
    showAtTill: false, printOnLabel: false, showInIntake: false, options: [], scope: [] })
  const sup = await post(page, '/api/v1/admin/suppliers', { name: SUP })
  const prod = await post(page, '/api/v1/admin/products', { name: `SBF part ${tag}`, nameLocal: '', barcode: '', categoryId: cat.id, unitId: 1,
    costPrice: '40', sellingPrice: '60', wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0,
    repriceStock: false, dupSourceName: '' })
  expect((await page.request.put(`${base}/values`, { data: [{ productId: prod.id, fieldId: field.id, value: VALUE }] })).ok()).toBe(true)

  try {
    await page.goto('/admin/x/productplus/suppliers')
    await page.getByRole('group', { name: 'Field that gives the supplier' }).getByRole('button', { name: BRAND }).click()
    await page.getByLabel('Search values').fill(VALUE)
    await page.getByRole('button', { name: `Supplier for ${VALUE}` }).first().click()
    await page.getByLabel('Search suppliers').fill(SUP)
    await page.getByLabel('Search suppliers').press('Enter')
    await expect(page.getByRole('button', { name: `Supplier for ${VALUE}` }).first()).toContainText(SUP)

    const list = await (await page.request.get(`/api/v1/admin/products?supplier=${sup.id}`)).json() as { items: { id: number; supplierWhy?: string }[] }
    expect(list.items.find((i) => i.id === prod.id)?.supplierWhy).toBe(`${BRAND}: ${VALUE}`)
  } finally {
    await page.request.put(`${base}/supplier-rules/field`, { data: { fieldId: 0, confirm: true } })
  }
})
