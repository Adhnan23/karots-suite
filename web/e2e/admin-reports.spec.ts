import { expect, test, type Page } from './fixtures'
import { customerWithHistory, expensesWithHistory } from './shot-data'

// M5 reports, dashboard, activity, warranty. Needs `make e2e-seed`.
test.describe.configure({ mode: 'serial' })

async function admin(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}
/** Today's cash, card and credit sales to report on — milk only: the till
 * flows need the seeded soap lots left as they are. */
async function salesWithHistory(page: Page) {
  if (!(await (await page.request.get('/api/v1/till/drawer')).json()).summary.session)
    await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
  if (((await (await page.request.get('/api/v1/admin/sales?preset=today&q=')).json()) as unknown[]).length >= 3) return
  const milk = (await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()).id
  const cust = await customerWithHistory(page)
  const sale = (quantity: string, payments: object[], customerId?: number) => page.request.post('/api/v1/till/sales', { data: { saleType: 'retail',
    discountType: 'fixed', customerId, payments, items: [{ productId: milk, quantity, discountType: 'fixed' }] } })
  await sale('4', [{ method: 'cash', amount: '1000' }])
  await sale('2', [{ method: 'card', amount: '200' }])
  await sale('1', [{ method: 'credit', amount: '0' }], cust)
}

/** Sell an E2E Phone (stock topped up first) and claim it by receipt: a
 * warranty unit + claim (WR slip) to find. */
async function phoneClaim(page: Page): Promise<string> {
  const phone = await (await page.request.get('/api/v1/till/products/barcode/4790000000066')).json()
  await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: phone.id, newQuantity: '5', note: 'e2e top-up', batchId: 0 } })
  const sale = await (await page.request.post('/api/v1/till/sales', { data: { saleType: 'retail', discountType: 'fixed',
    payments: [{ method: 'cash', amount: '25000' }], items: [{ productId: phone.id, quantity: '1', discountType: 'fixed' }] } })).json()
  expect(sale.id, JSON.stringify(sale)).toBeTruthy()
  const claim = await page.request.post('/api/v1/till/warranty/receipt-replace', { data: { saleId: sale.id, productId: phone.id, reason: 'Dead' } })
  expect(claim.ok(), await claim.text()).toBe(true)
  return sale.receiptNo
}
const noSideScroll = (page: Page) => expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true)

const PAGES: [string, string | RegExp][] = [
  ['/admin', /^Good (morning|afternoon|evening)/], ['/admin/reports', 'Reports'],
  ['/admin/reports/sales', 'Sales'], ['/admin/reports/peak-hours', 'Peak hours'], ['/admin/reports/top-products', 'Top products'],
  ['/admin/reports/product-sales', 'Product sales'], ['/admin/reports/sales-by-cashier', 'Sales by cashier'], ['/admin/reports/returns', 'Returns'],
  ['/admin/reports/customer-dues', 'Customer dues'], ['/admin/reports/supplier-dues', 'Supplier dues'], ['/admin/reports/supplier-spend', 'Spend by supplier'],
  ['/admin/reports/purchases', 'Purchases'], ['/admin/reports/profit-by-category', 'Profit by category'], ['/admin/reports/expenses', 'Expenses by category'],
  ['/admin/reports/tax', 'Tax'], ['/admin/reports/inventory', 'Inventory valuation'], ['/admin/reports/count-sheet', 'Stock count sheet'],
  ['/admin/reports/topup', 'Top-up and reorder'], ['/admin/reports/losses', 'Losses and recovery'], ['/admin/reports/recipe-variance', 'Recipe variance'],
  ['/admin/reports/service-profit', 'Service profit'], ['/admin/activity', 'Activity'], ['/admin/warranty', 'Warranty'],
  ['/admin/tills', 'Till sessions'], ['/admin/stock/batches', 'Batches'], ['/admin/finance', 'Finance'], ['/admin/reorder', 'Reorder'],
]

for (const width of [1366, 360]) {
  test(`every report renders, with data and with an empty range, at ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: width < 768 ? 780 : 768 })
    await admin(page)
    await salesWithHistory(page)
    await expensesWithHistory(page)
    for (const [path, heading] of PAGES) {
      await page.goto(path)
      await expect(page.getByRole('heading', { level: 1, name: heading })).toBeVisible()
      await expect(page.getByText('Something went wrong')).toHaveCount(0)
      if (width < 768) await noSideScroll(page)
    }
    // An empty range shows zeros and an empty state, not an error.
    await page.goto('/admin/reports/sales?preset=custom&from=2001-01-01&to=2001-01-02')
    await expect(page.getByText('No sales in this period')).toBeVisible()
  })
}

test('filters live in the URL, exports download, a receipt opens its lines', async ({ page }) => {
  await admin(page)
  await salesWithHistory(page)
  await page.goto('/admin/reports/sales?preset=today')
  await page.getByLabel('Payment').selectOption('cash')
  await page.getByLabel('From hour').selectOption('0')
  await expect(page).toHaveURL(/method=cash/)
  await page.reload()
  await expect(page.getByLabel('Payment')).toHaveValue('cash')
  await expect(page.getByLabel('From hour')).toHaveValue('0')
  await page.getByLabel('Receipts').locator('visible=true').getByRole('button').first().click()
  await expect(page.getByRole('dialog')).toBeVisible()

  for (const url of ['/api/v1/admin/sales/export?preset=today&format=csv', '/api/v1/admin/reports/top-products?preset=today&format=xlsx',
    '/api/v1/admin/reports/customer-dues?format=ods', '/api/v1/admin/reports/inventory?format=csv', '/api/v1/admin/finance?preset=today&format=csv',
    '/api/v1/admin/activity?preset=today&format=csv']) {
    const r = await page.request.get(url)
    expect(r.status(), url).toBe(200)
    expect(r.headers()['content-disposition'], url).toContain('attachment')
  }
})

test('top products → product sales; profit by category drills into a branch', async ({ page }) => {
  await admin(page)
  await salesWithHistory(page)
  await page.goto('/admin/reports/top-products?preset=today')
  await expect(page.getByText(/make 80% of your revenue|make .* of your revenue/)).toBeVisible()
  await page.getByLabel('Products').locator('visible=true').getByRole('button').first().click()
  await expect(page).toHaveURL(/product-sales\?.*product=\d+/)
  await expect(page.getByRole('heading', { name: 'Units sold' })).toBeVisible()
  await page.goto('/admin/reports/profit-by-category?preset=today')
  await page.getByLabel('Categories').locator('visible=true').getByRole('button').first().click()
  await expect(page).toHaveURL(/category=\d+/)
})

test('losses: a damage write-off is recovered from the supplier', async ({ page }) => {
  await admin(page)
  const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
  expect((await page.request.post('/api/v1/admin/stock/damage', { data: { productId: milk.id, quantity: '1', reason: 'damage', note: 'e2e dropped', batchId: 0 } })).ok()).toBe(true)
  await page.goto('/admin/reports/losses?preset=today')
  await page.getByRole('button', { name: 'Recover' }).first().click()
  await page.getByRole('button', { name: 'Nothing', exact: true }).click()
  await page.getByRole('button', { name: 'Record', exact: true }).click()
  await expect(page.locator('[data-sonner-toast]').filter({ hasText: 'Recovery recorded' }).first()).toBeVisible()
  await expect(page.getByText('written off').first()).toBeVisible()
})

test('warranty register opens a unit; activity filters by who; the dashboard range switches', async ({ page }) => {
  await admin(page)
  await customerWithHistory(page)
  if (!(await (await page.request.get('/api/v1/till/drawer')).json()).summary.session)
    await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
  const receiptNo = await phoneClaim(page)
  await page.goto('/admin/warranty?status=all')
  const units = page.getByLabel('Units').locator('visible=true').getByRole('button')
  await expect(units.first()).toBeVisible()
  await page.getByLabel('Claim by receipt').fill(receiptNo)
  await page.getByLabel('Claim by receipt').press('Enter')
  await expect(page.getByRole('dialog', { name: new RegExp(`Claim on ${receiptNo}`) })).toBeVisible()
  await page.keyboard.press('Escape')
  await page.goto('/admin/activity?preset=today')
  await expect(page.getByLabel('Activity').locator('visible=true').getByRole('button').first().or(page.getByLabel('Activity').locator('visible=true').locator('tr, li').first())).toBeVisible()
  const who = page.getByLabel('Who')
  await who.selectOption({ index: 1 })
  await expect(page).toHaveURL(/user=\d+/)

  await page.goto('/admin')
  await page.getByRole('group', { name: 'Chart range' }).getByRole('button', { name: '30 days' }).click()
  await expect(page).toHaveURL(/days=30/)
  await expect(page.getByText('Top sellers, last 30 days')).toBeVisible()
})

test('receipts: credit and warranty slips open in the admin and at the counter', async ({ page }) => {
  await admin(page)
  // A drawer, a credit payment (DP-) and a by-receipt warranty claim to find.
  if (!(await (await page.request.get('/api/v1/till/drawer')).json()).summary.session)
    await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
  const cust = await customerWithHistory(page)
  const pay = await page.request.post(`/api/v1/till/customers/${cust}/payment`, { data: { amount: '50', method: 'cash', note: 'e2e', accountId: 0, reference: '' } })
  expect(pay.ok(), await pay.text()).toBe(true)
  await phoneClaim(page)

  for (const base of ['/admin/receipts', '/till/receipts']) {
    await page.goto(base)
    for (const kind of ['Credit', 'Warranty']) {
      await page.getByRole('group', { name: 'Receipt kind' }).getByRole('button', { name: kind, exact: true }).click()
      const rows = page.getByLabel('Receipts').locator('visible=true').getByRole('button')
      await expect(rows.first(), `${base} ${kind}`).toBeVisible()
      await rows.first().click()
      await expect(page.getByText(new RegExp(kind === 'Credit' ? 'AMOUNT|PAID|Print again' : 'WARRANTY|Replaced|Print again', 'i')).first(), `${base} ${kind} slip`).toBeVisible()
      await page.keyboard.press('Escape')
    }
  }
})
