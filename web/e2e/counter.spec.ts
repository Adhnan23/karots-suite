import { expect, test, type Page } from './fixtures'

// Golden counter flows (M1b). Needs `make e2e-seed`. Serial: one admin drawer.
test.describe.configure({ mode: 'serial' })

async function open(page: Page, tab: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 4 }, sourceLockerId: 0 } })
  await page.goto(`/till/${tab}`)
  await page.waitForLoadState('networkidle')
}

const product = async (page: Page, code: string) => (await page.request.get(`/api/v1/till/products/barcode/${code}`)).json()
async function sell(page: Page, lines: [string, string][], payments: object[], customerId?: number) {
  const items = await Promise.all(lines.map(async ([code, qty]) => ({ productId: (await product(page, code)).id, quantity: qty, discountType: 'fixed' })))
  return (await page.request.post('/api/v1/till/sales', { data: { saleType: 'retail', discountType: 'fixed', customerId, items, payments } })).json()
}
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()

test('the cart survives a trip to another tab', async ({ page }) => {
  await open(page, '')
  const s = page.getByLabel('Scan a barcode or search products')
  await s.fill('4790000000011')
  await s.press('Enter')
  await expect(page.getByLabel('Quantity of E2E Milk')).toHaveValue('1')
  await page.getByRole('link', { name: 'Returns' }).click()
  await expect(page.getByLabel('Find a sale')).toBeVisible()
  await page.getByRole('link', { name: 'Sell' }).click()
  await expect(page.getByLabel('Quantity of E2E Milk')).toHaveValue('1')
})

test('expense from the drawer lands in today’s list', async ({ page }) => {
  await open(page, 'expenses')
  await page.getByRole('button', { name: 'Transport', exact: true }).click()
  await page.getByLabel('Amount').fill('250')
  await page.getByLabel('Note').fill('e2e tuk')
  await page.getByRole('button', { name: /Record expense/ }).click()
  await toast(page, /Expense recorded/)
  await expect(page.getByText('e2e tuk')).toBeVisible()
})

test('write-off as shop use takes stock out', async ({ page }) => {
  await open(page, 'write-off')
  const before = Number((await product(page, '4790000000028')).stockQty)
  await page.getByLabel('Find a product').fill('4790000000028')
  await page.getByLabel('Find a product').press('Enter')
  await page.getByRole('radio', { name: /Shop use/ }).click()
  await page.getByLabel('Quantity').fill('1')
  await page.getByRole('button', { name: /out of stock/ }).click()
  await toast(page, /shop use/)
  expect(Number((await product(page, '4790000000028')).stockQty)).toBe(before - 1)
})

test('convert opens a crate into 12 milk', async ({ page }) => {
  await open(page, 'convert')
  const before = Number((await product(page, '4790000000011')).stockQty)
  await page.getByRole('button', { name: /Milk Crate/ }).click()
  await page.getByRole('button', { name: /Open 1 pcs/ }).click()
  await toast(page, /Opened 1/)
  expect(Number((await product(page, '4790000000011')).stockQty)).toBe(before + 12)
})

test('return by receipt refunds cash', async ({ page }) => {
  await open(page, 'returns')
  const sale = await sell(page, [['4790000000011', '2']], [{ method: 'cash', amount: '200' }])
  await page.getByLabel('Find a sale').fill(sale.receiptNo)
  await page.getByLabel('Find a sale').press('Enter')
  await page.getByRole('button', { name: 'More E2E Milk' }).click()
  await page.getByRole('button', { name: /Refund Rs\. 100\.00 cash/ }).click()
  await toast(page, /Give back Rs\. 100\.00/)
  await expect(page.getByText('1 already back')).toBeVisible()
})

test('credit: collect what a customer owes', async ({ page }) => {
  await open(page, 'credit')
  const cust = (await (await page.request.get('/api/v1/till/customers')).json()).find((c: { name: string }) => c.name === 'E2E Credit')
  await sell(page, [['4790000000011', '1']], [{ method: 'credit', amount: '100' }], cust.id)
  await page.reload()
  await page.getByRole('button', { name: /E2E Credit/ }).click()
  await page.getByRole('button', { name: /^Full / }).click()
  await page.getByRole('button', { name: /^Collect/ }).click()
  await toast(page, /collected from E2E Credit/)
})

test('receipts: a sale shows as paper', async ({ page }) => {
  await open(page, 'receipts')
  const sale = await sell(page, [['4790000000011', '1']], [{ method: 'cash', amount: '100' }])
  await page.getByLabel('Find a sale').fill(sale.receiptNo)
  await page.getByLabel('Find a sale').press('Enter')
  await expect(page.getByText('TOTAL', { exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: /Open to print/ })).toHaveAttribute('href', `/print/receipt/${sale.id}`)
})

test('warranty: swap a serial, keeping the cover', async ({ page }) => {
  await open(page, 'warranty')
  await page.getByLabel('Serial number').fill('E2E-SN-0001')
  await page.getByLabel('Serial number').press('Enter')
  await page.getByLabel('New serial').fill(`E2E-SN-${Date.now()}`)
  await page.getByRole('button', { name: 'Swap for the new unit' }).click()
  await toast(page, /Swapped E2E-SN-0001/)
  await expect(page.getByRole('heading', { name: 'E2E Phone' })).toBeVisible()
})

test('supplier: pay part of an old debt, then take in a delivery', async ({ page }) => {
  await open(page, 'suppliers')
  await page.getByRole('button', { name: /E2E Distributors/ }).click()
  await page.getByLabel('Amount').fill('500')
  await page.getByRole('button', { name: /^Pay Rs\. 500\.00/ }).click()
  await toast(page, /Paid Rs\. 500\.00 to E2E Distributors/)
  await expect(page.getByText(/We owe Rs\. 1,000\.00/)).toBeVisible()

  await page.getByRole('button', { name: 'Receive stock' }).click()
  await page.getByLabel('Add a product').fill('4790000000011')
  await page.getByLabel('Add a product').press('Enter')
  await page.getByLabel('Quantity of E2E Milk').fill('6')
  await page.getByLabel('Cost of E2E Milk').fill('80')
  await page.getByLabel('Discount on E2E Milk').fill('10')
  await page.getByRole('group', { name: 'Discount type for E2E Milk' }).getByRole('button', { name: '%' }).click()
  await page.getByRole('button', { name: /Receive Rs\. 432\.00 of stock/ }).click()
  await toast(page, /Received Rs\. 432\.00 of stock/)
  await expect(page.getByText(/We owe Rs\. 1,432\.00/)).toBeVisible()
})
