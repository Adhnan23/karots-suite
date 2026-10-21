import { expect, test, type Page } from './fixtures'

// Golden counter flows (M1b). Needs `make e2e-seed`. Serial: one admin drawer.
test.describe.configure({ mode: 'serial' })

async function open(page: Page, tab: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000003') // E2E Manager: sees every tab
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
/** An action that may make a slip: with "ask before printing" on, the Print / Skip
 * prompt shows the message and the receipt as it will print — check it, skip it.
 * Actions with no slip still just toast. */
async function done(page: Page, text: RegExp) {
  const ask = page.getByRole('dialog', { name: 'Print the receipt?' }).filter({ hasText: text })
  await expect(ask.or(page.locator('[data-sonner-toast]').filter({ hasText: text })).first()).toBeVisible()
  if (await ask.isVisible()) {
    await expect(ask.getByLabel('Receipt preview')).toContainText('Karots Business Suite')
    await ask.getByRole('button', { name: 'Skip' }).click()
    await expect(ask).toBeHidden()
  }
}

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
  await done(page, /Expense recorded/)
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
  await done(page, /shop use/)
  expect(Number((await product(page, '4790000000028')).stockQty)).toBe(before - 1)
})

test('convert opens a crate into 12 milk', async ({ page }) => {
  await open(page, 'convert')
  const before = Number((await product(page, '4790000000011')).stockQty)
  await page.getByRole('button', { name: /Milk Crate/ }).click()
  await page.getByRole('button', { name: /Open 1 pcs/ }).click()
  await done(page, /Opened 1/)
  expect(Number((await product(page, '4790000000011')).stockQty)).toBe(before + 12)
})

test('return by receipt refunds cash', async ({ page }) => {
  await open(page, 'returns')
  const sale = await sell(page, [['4790000000011', '2']], [{ method: 'cash', amount: '200' }])
  await page.getByLabel('Find a sale').fill(sale.receiptNo)
  await page.getByLabel('Find a sale').press('Enter')
  await page.getByRole('button', { name: 'More E2E Milk' }).click()
  await page.getByRole('button', { name: /Refund Rs\. 100\.00 cash/ }).click()
  await done(page, /Give back Rs\. 100\.00/)
  await expect(page.getByText('1 already back')).toBeVisible()
})

test('credit: collect what a customer owes', async ({ page }) => {
  await open(page, 'credit')
  // Its own customer: till.spec's over-limit flow depends on E2E Credit's balance, and specs run in parallel.
  const name = `E2E Counter ${Date.now()}`
  const cr = await page.request.post('/api/v1/till/customers', { data: { name, phone: `07${String(Date.now()).slice(-8)}`, creditLimit: '1000' } })
  const cust = (await cr.json()).customer
  await sell(page, [['4790000000011', '1']], [{ method: 'credit', amount: '100' }], cust.id)
  await page.reload()
  await page.getByRole('button', { name: new RegExp(name) }).click()
  await page.getByRole('button', { name: /^Full / }).click()
  await page.getByRole('button', { name: /^Collect/ }).click()
  await done(page, new RegExp(`collected from ${name}`))
})

test('receipts: a sale shows as paper', async ({ page }) => {
  await open(page, 'receipts')
  const sale = await sell(page, [['4790000000011', '1']], [{ method: 'cash', amount: '100' }])
  await page.getByLabel('Find a sale').fill(sale.receiptNo)
  await page.getByLabel('Find a sale').press('Enter')
  // The slip as it prints (Classic: the decoded printer lines).
  const slip = page.getByRole('article', { name: 'Receipt preview' })
  await expect(slip).toContainText(sale.receiptNo)
  await expect(slip).toContainText(/TOTAL\s+Rs\. 100\.00/)
  await expect(page.getByRole('link', { name: /Print A4/ })).toHaveAttribute('href', `/print/receipt/${sale.id}`)
})

test('warranty: swap a serial, keeping the cover', async ({ page }) => {
  await open(page, 'warranty')
  await page.getByLabel('Serial number').fill('E2E-SN-0001')
  await page.getByLabel('Serial number').press('Enter')
  await page.getByLabel('New serial').fill(`E2E-SN-${Date.now()}`)
  await page.getByRole('button', { name: 'Swap for the new unit' }).click()
  await done(page, /Swapped E2E-SN-0001/)
  await expect(page.getByRole('heading', { name: 'E2E Phone' })).toBeVisible()
})

test('supplier: pay part of an old debt, then take in a delivery', async ({ page }) => {
  await open(page, 'suppliers')
  await page.getByRole('button', { name: /E2E Distributors/ }).click()
  await page.getByLabel('Amount').fill('500')
  await page.getByRole('button', { name: /^Pay Rs\. 500\.00/ }).click()
  await done(page, /Paid Rs\. 500\.00 to E2E Distributors/)
  await expect(page.getByText(/We owe Rs\. 1,000\.00/)).toBeVisible()

  await page.getByRole('button', { name: 'Receive stock' }).click()
  await page.getByLabel('Add a product').fill('4790000000011')
  await page.getByLabel('Add a product').press('Enter')
  await page.getByLabel('Quantity of E2E Milk').fill('6')
  await page.getByLabel('Cost of E2E Milk').fill('80')
  await page.getByLabel('Discount on E2E Milk').fill('10')
  await page.getByRole('group', { name: 'Discount type for E2E Milk' }).getByRole('button', { name: '%' }).click()
  await page.getByRole('button', { name: /Receive Rs\. 432\.00 of stock/ }).click()
  await done(page, /Received Rs\. 432\.00 of stock/)
  await expect(page.getByText(/We owe Rs\. 1,432\.00/)).toBeVisible()
})

test('supplier: place an order, then receive it', async ({ page }) => {
  await open(page, 'suppliers')
  await page.getByRole('button', { name: /E2E Distributors/ }).click()
  await page.getByRole('button', { name: 'Order', exact: true }).click()
  await page.getByLabel('Add a product').fill('4790000000035')
  await page.getByLabel('Add a product').press('Enter')
  await page.getByLabel('Quantity of E2E Yoghurt').fill('12')
  await page.getByRole('button', { name: /Place order · 1 item/ }).click()
  await expect(page.getByText(/Order #\d+ placed/)).toBeVisible()
  await expect(page.getByRole('link', { name: /Print the order/ })).toHaveAttribute('href', /\/print\/order\/\d+/)

  await page.getByRole('button', { name: 'Receive stock' }).click()
  await page.getByRole('button', { name: /^#\d+/ }).first().click()
  await expect(page.getByLabel('Quantity of E2E Yoghurt')).toHaveValue('12')
  await page.getByRole('button', { name: /Receive Rs\. .* of stock/ }).click()
  await done(page, /Received .* of stock/)
})

test('receipts: cash moves and credit payments are listed', async ({ page }) => {
  await open(page, 'receipts')
  await page.getByRole('group', { name: 'Receipt kind' }).getByRole('button', { name: 'Cash' }).click()
  await expect(page.getByRole('list', { name: 'Receipts' }).getByRole('button').first()).toBeVisible()
  await page.getByRole('list', { name: 'Receipts' }).getByRole('button').first().click()
  await expect(page.getByRole('article', { name: 'Receipt preview' })).toContainText(/Amount\s+Rs\./)
  await page.getByRole('group', { name: 'Receipt kind' }).getByRole('button', { name: 'Credit' }).click()
  await expect(page.getByRole('list', { name: 'Receipts' }).getByRole('button').first()).toBeVisible()
})

test('the shift read (X) shows sales and movements', async ({ page }) => {
  await open(page, 'receipts')
  const popup = page.waitForEvent('popup')
  await page.getByRole('link', { name: /My shift so far/ }).click()
  const z = await popup
  await expect(z.getByText(/Shift so far \(X read\)/)).toBeVisible()
  await expect(z.getByText('Cash movements')).toBeVisible()
})

// ---- M1 features ported earlier, flow-checked in M2 ----

test('credit: hand back a customer’s advance from the drawer', async ({ page }) => {
  await open(page, 'credit')
  const cust = (await (await page.request.get('/api/v1/till/customers')).json()).find((c: { name: string }) => c.name === 'E2E Credit')
  // Overpay what they owe, so they hold a Rs. 300 advance.
  const owes = Math.max(0, Number(cust.outstanding))
  await page.request.post(`/api/v1/till/customers/${cust.id}/payment`, { data: { amount: String(owes + 300), method: 'cash', accountId: 0, reference: '', note: 'e2e', deferDrawerKick: true } })
  await page.reload()
  await page.getByRole('group', { name: 'Show' }).getByRole('button', { name: 'Advance' }).click()
  await page.getByRole('button', { name: /E2E Credit/ }).click()
  await page.getByRole('group', { name: 'Action' }).getByRole('button', { name: 'Refund advance' }).click()
  await expect(page.getByLabel('Amount')).toHaveValue('300.00')
  await page.getByRole('button', { name: /Refund Rs\. 300\.00 from the drawer/ }).click()
  await done(page, /advance refunded/)
  const after = (await (await page.request.get('/api/v1/till/customers')).json()).find((c: { name: string }) => c.name === 'E2E Credit')
  expect(Number(after.outstanding)).toBe(0)
})

test('supplier: add one at the counter, overpay, take the advance back', async ({ page }) => {
  await open(page, 'suppliers')
  const name = `E2E Quick ${Date.now().toString(36).slice(-5)}`
  await page.getByRole('button', { name: 'New', exact: true }).click()
  await page.getByLabel('Supplier name').fill(name)
  await page.getByLabel('Supplier phone').fill('0110000000')
  await page.getByLabel('Supplier name').press('Enter')
  await page.getByRole('button', { name: new RegExp(name) }).click()
  await page.getByLabel('Amount').fill('400')
  await page.getByRole('button', { name: /^Pay Rs\. 400\.00/ }).click()
  await done(page, new RegExp(`Paid Rs\\. 400\\.00 to ${name}`))
  await page.getByRole('group', { name: 'Direction' }).getByRole('button', { name: 'They pay us back' }).click()
  await expect(page.getByLabel('Amount')).toHaveValue('400.00')
  await page.getByRole('button', { name: /Take Rs\. 400\.00 back/ }).click()
  await done(page, new RegExp(`Received Rs\\. 400\\.00 back from ${name}`))
})

test('supplier: a new product met on a delivery, and a wanted item on an order', async ({ page }) => {
  await open(page, 'suppliers')
  const tag = Date.now().toString(36).slice(-5)
  await page.getByRole('button', { name: /E2E Distributors/ }).click()
  await page.getByRole('button', { name: 'Receive stock' }).click()
  await page.getByLabel('Add a product').fill(`Jaggery ${tag}`)
  await page.getByRole('button', { name: new RegExp(`New product “Jaggery ${tag}”`) }).click()
  await page.getByLabel('New product cost').fill('150')
  await page.getByLabel('New product selling price').fill('210')
  await page.getByRole('button', { name: 'Add', exact: true }).click()
  await page.getByLabel(`Quantity of Jaggery ${tag}`).fill('5')
  await page.getByRole('button', { name: /Receive Rs\. 750\.00 of stock/ }).click()
  await done(page, /Received Rs\. 750\.00 of stock/)
  expect(Number((await (await page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(`Jaggery ${tag}`)}`)).json()).items[0].stockQty)).toBe(5)

  await page.getByRole('button', { name: 'Order', exact: true }).click()
  await page.getByLabel('Add a product').fill(`Treacle ${tag}`)
  await page.getByRole('button', { name: new RegExp(`New product “Treacle ${tag}”`) }).click()
  await page.getByRole('button', { name: 'Add', exact: true }).click()
  await page.getByLabel(`Quantity of Treacle ${tag}`).fill('6')
  await page.getByRole('button', { name: /Place order · 1 item/ }).click()
  await expect(page.getByText(/Order #\d+ placed/)).toBeVisible()
  // A wanted item waits in Items to review for its real details.
  const review = await (await page.request.get('/api/v1/admin/products/review')).json()
  expect(review.some((p: { name: string }) => p.name === `Treacle ${tag}`)).toBeTruthy()
})

test('warranty: replace an unserialled item by its receipt', async ({ page }) => {
  await open(page, 'warranty')
  const phone = await product(page, '4790000000066')
  const sale = await (await page.request.post('/api/v1/till/sales', { data: {
    saleType: 'retail', discountType: 'fixed', payments: [{ method: 'cash', amount: '25000' }],
    items: [{ productId: phone.id, quantity: '1', discountType: 'fixed' }],
  } })).json()
  await page.getByRole('group', { name: 'Find by' }).getByRole('button', { name: 'By receipt' }).click()
  await page.getByLabel('Receipt number').fill(sale.receiptNo)
  await page.getByLabel('Receipt number').press('Enter')
  await page.getByRole('button', { name: 'Replace one E2E Phone' }).click()
  await done(page, /Replacement recorded/)
  await expect(page.getByText(/1 replaced/)).toBeVisible()
})
