import { expect, test, type Page } from './fixtures'

// Scan a slip to find it (spec 2026-10-21): a scanned receipt number opens that
// slip — at Sell it jumps to Receipts; on Returns it picks the sale in place.
test.describe.configure({ mode: 'serial' })

async function signIn(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000003') // E2E Manager: till + admin products
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 4 }, sourceLockerId: 0 } })
}

async function newSale(page: Page): Promise<string> {
  const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
  const res = await page.request.post('/api/v1/till/sales', { data: { saleType: 'retail', discountType: 'fixed',
    items: [{ productId: milk.id, quantity: '1', discountType: 'fixed' }], payments: [{ method: 'cash', amount: '500' }] } })
  expect(res.ok(), await res.text()).toBe(true)
  return (await res.json()).receiptNo
}

const scan = async (page: Page, text: string) => {
  const box = page.getByLabel('Scan a barcode or search products')
  await box.fill(text)
  await box.press('Enter')
}

test('at Sell a scanned receipt opens it on Receipts', async ({ page }) => {
  await signIn(page)
  const no = await newSale(page)
  await newSale(page) // a newer one, so the find has to pick by number
  await page.goto('/till')
  await scan(page, no)
  await expect(page).toHaveURL(new RegExp(`/till/receipts\\?no=${no}$`))
  await expect(page.getByRole('list', { name: 'Sales' }).getByRole('button', { pressed: true })).toContainText(no)
  await expect(page.getByRole('button', { name: /Print again/ })).toBeVisible()
})

test('on Returns a scanned receipt picks that sale', async ({ page }) => {
  await signIn(page)
  const no = await newSale(page)
  await newSale(page)
  await page.goto('/till/returns')
  const find = page.getByLabel('Find a sale')
  await find.fill(no)
  await find.press('Enter')
  await expect(page.getByRole('list', { name: 'Sales' }).getByRole('button', { pressed: true })).toContainText(no)
})

test('a slip number is never a product barcode', async ({ page }) => {
  await signIn(page)
  const cat = (await (await page.request.get('/api/v1/admin/categories')).json()).find((c: { name: string }) => c.name === 'E2E Goods')
  await page.request.post('/api/v1/admin/products', { data: {
    name: 'E2E Slip Look-alike', nameLocal: '', barcode: 'S-99999', categoryId: cat.id, unitId: 1, costPrice: '40', sellingPrice: '60', wholesalePrice: '',
    taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } }) // a second run finds it already there
  expect((await page.request.get('/api/v1/till/products/barcode/S-99999')).ok()).toBe(true)
  await page.goto('/till')
  await scan(page, 'S-99999')
  await expect(page).toHaveURL(/\/till\/receipts\?no=S-99999$/)
  await page.goto('/till')
  await expect(page.getByRole('region', { name: 'Cart' }).getByLabel('Quantity of E2E Slip Look-alike')).toHaveCount(0)
})
