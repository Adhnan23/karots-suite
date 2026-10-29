import { expect, freshProduct, test, type Page, slipAsked } from './fixtures'

// M3 purchasing admin flows (suppliers, purchases, returns, reorder). Needs `make e2e-seed`.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString(36).slice(-5)

async function admin(page: Page, path: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(path)
  await page.waitForLoadState('networkidle')
}
/** Pick the <option> whose text contains `text` (labels carry live balances). */
async function pick(page: Page, label: string, text: string) {
  const sel = page.getByLabel(label)
  await sel.selectOption((await sel.locator('option', { hasText: text }).first().getAttribute('value'))!)
}
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()

test('suppliers: add one, pay old debt in cash from the safe, correct the old balance, statement', async ({ page }) => {
  await admin(page, '/admin/suppliers')
  const name = `Lanka Foods ${tag}`
  await page.getByRole('button', { name: 'New supplier' }).click()
  await page.getByLabel('Name').fill(name)
  await page.getByLabel('Phone').fill('0712223344')
  await page.getByLabel('Credit days').fill('30')
  await page.getByLabel('Already owed').fill('1000')
  await page.getByRole('button', { name: 'Add supplier' }).click()
  await toast(page, new RegExp(`${name} added`))
  await page.waitForURL(/\/admin\/suppliers\/\d+/)
  await expect(page.getByText('↑ We owe them')).toBeVisible()
  await page.getByRole('button', { name: 'Edit' }).click()
  await page.getByLabel('Contact person').fill('Nimal')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await toast(page, /Supplier updated/)
  await expect(page.getByText(/Nimal · 0712223344/)).toBeVisible()

  await page.getByRole('button', { name: '↑ Pay' }).click()
  await page.getByLabel('Amount paid').fill('400')
  await pick(page, 'Pay the cash from', 'E2E Safe')
  await page.getByRole('button', { name: /↑ Pay Rs\. 400\.00/ }).click()
  await slipAsked(page, new RegExp(`Paid Rs\\. 400\\.00 to ${name}`))
  await expect(page.getByLabel('Payments and refunds').getByText('↑ Paid to them')).toBeVisible()

  await page.getByRole('button', { name: 'More' }).click()
  await page.getByRole('menuitem', { name: 'Old balance' }).click()
  await page.getByLabel('Old balance', { exact: true }).fill('500')
  await page.getByLabel('Why').fill('Found a paid bill')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await toast(page, /Old balance corrected/)
  await expect(page.getByText('incl. old balance Rs. 500.00')).toBeVisible()

  const id = page.url().split('/').pop()
  await page.goto(`/print/statement/supplier/${id}`)
  await expect(page.getByText('Supplier statement')).toBeVisible()
  await expect(page.getByRole('cell', { name: 'Payment' })).toBeVisible()
})

test('suppliers: in credit → take a refund; move products; disable and re-enable', async ({ page }) => {
  await admin(page, '/admin/suppliers')
  const mk = async (n: string, opening: string) => (await (await page.request.post('/api/v1/admin/suppliers', { data: { name: n, contact: '', phone: '', address: '', creditDays: 0, openingBalance: opening } })).json()).id as number
  const a = await mk(`Old Distributor ${tag}`, '0')
  const b = await mk(`New Distributor ${tag}`, '0')
  await page.request.post(`/api/v1/admin/suppliers/${a}/payment`, { data: { amount: '300', mode: 'pay', allocations: [], advance: '', oldDebt: '', method: 'online', reference: '', note: '', source: '' } })
  await page.goto(`/admin/suppliers/${a}`)
  await expect(page.getByText('↓ They owe us')).toBeVisible()
  await page.getByRole('button', { name: '↓ Take refund' }).click()
  await page.getByLabel('Amount received').fill('100')
  await pick(page, 'Put the cash into', 'E2E Safe')
  await page.getByRole('button', { name: /Record Rs\. 100\.00/ }).click()
  await slipAsked(page, /Received Rs\. 100\.00 back/)

  const cat = (await (await page.request.get('/api/v1/admin/categories')).json()).find((c: { name: string }) => c.name === 'E2E Goods')
  await page.request.post('/api/v1/admin/products', { data: {
    name: `Distributed ${tag}`, nameLocal: '', barcode: '', categoryId: cat.id, unitId: 1, costPrice: '40', sellingPrice: '60', wholesalePrice: '', taxRate: '',
    reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: a, repriceStock: false, dupSourceName: '',
  } })
  await page.reload()
  await page.getByRole('button', { name: 'More' }).click()
  await page.getByRole('menuitem', { name: 'Move products (1)' }).click()
  await pick(page, 'Move all to', `New Distributor ${tag}`)
  await page.getByRole('button', { name: 'Move them' }).click()
  await toast(page, /Moved 1 product/)
  await page.getByRole('button', { name: 'More' }).click()
  await expect(page.getByRole('menuitem', { name: 'Move products (0)' })).toBeVisible()
  await page.keyboard.press('Escape')

  await page.getByRole('button', { name: 'More' }).click()
  await page.getByRole('menuitem', { name: 'Disable' }).click()
  await page.getByRole('button', { name: 'Disable', exact: true }).click()
  await toast(page, /disabled/)
  await page.getByRole('button', { name: 'Re-enable' }).click()
  await toast(page, /re-enabled/)
})

test('purchases: order → edit → receive short with an extra item, margin fix, pay from the safe, rest kept on order; PO + GRN print', async ({ page }) => {
  await admin(page, '/admin/purchases')
  const sup = `Order Supplier ${tag}`
  await page.request.post('/api/v1/admin/suppliers', { data: { name: sup, contact: '', phone: '', address: '', creditDays: 14, openingBalance: '' } })
  const tea = `PO Tea ${tag}`
  const rice = `PO Rice ${tag}`
  const salt = `PO Salt ${tag}`
  await freshProduct(page, tea) // cost 40, sells 60
  await freshProduct(page, rice)
  await freshProduct(page, salt)
  const add = async (name: string) => {
    await page.getByLabel('Add a product').fill(name)
    await page.getByRole('button', { name: new RegExp(`^${name}`) }).click()
  }
  await page.getByRole('link', { name: 'New order' }).click()
  await pick(page, 'Supplier', sup)
  await add(tea)
  await page.getByLabel(`Quantity of ${tea}`).fill('10')
  await expect(page.getByLabel(`Cost of ${tea}`)).toHaveValue('40.00') // prefilled from the product
  await add(rice)
  await page.getByLabel(`Quantity of ${rice}`).fill('5')
  await page.getByRole('button', { name: /Place order/ }).click()
  await toast(page, /Order #\d+ placed/)
  await expect(page.getByText('ordered', { exact: true })).toBeVisible()
  const id = page.url().split('/').pop()!

  await page.getByRole('link', { name: 'Edit' }).click()
  await page.getByLabel(`Quantity of ${tea}`).fill('12')
  await page.getByRole('button', { name: 'Save order' }).click()
  await toast(page, /Order updated/)
  await expect(page.getByText(/12 pcs/)).toBeVisible()

  await page.goto(`/print/po?ids=${id}`)
  await expect(page.getByText(`Purchase order #${id}`).last()).toBeVisible()
  await page.goto(`/admin/purchases/${id}`)

  await page.getByRole('link', { name: 'Receive' }).click()
  await page.getByLabel(`Quantity of ${tea}`).fill('8')
  await expect(page.getByText('4 short')).toBeVisible()
  await page.getByLabel(`Cost of ${tea}`).fill('100') // dearer than it sells for → the guard steps in
  await page.getByRole('button', { name: 'Use Rs. 150.00' }).click() // keeps its 1.5× markup
  await expect(page.getByText('Sells under its usual markup')).toHaveCount(0)
  await page.getByLabel(`Quantity of ${rice}`).fill('0') // none of it came
  await add(salt) // came, but wasn't ordered
  await page.getByLabel(`Cost of ${salt}`).fill('45')
  await page.getByLabel('Their invoice no').fill(`INV-${tag}`)
  await page.getByLabel('Pay now').fill('100')
  await pick(page, 'Pay the cash from', 'E2E Safe')
  await expect(page.getByText("Keep what didn't come on a new order")).toBeVisible()
  await page.getByRole('button', { name: /Receive Rs\..* of stock/ }).click()
  await slipAsked(page, /paid Rs\. 100\.00/)
  await expect(page.getByText('4 short')).toBeVisible()
  await expect(page.getByText('not on the order')).toBeVisible()

  await page.goto(`/print/grn/${id}`)
  await expect(page.getByText('Goods received note')).toBeVisible()
  await expect(page.getByText(`their invoice INV-${tag}`)).toBeVisible()

  const orders = await (await page.request.get(`/api/v1/admin/purchases?tab=orders&q=${encodeURIComponent(sup)}`)).json()
  expect(orders.total).toBe(1) // what didn't come stays on order: 4 tea + all 5 rice
  const rest = await (await page.request.get(`/api/v1/admin/purchases/${orders.items[0].id}`)).json()
  expect(rest.lines.map((l: { name: string; quantity: string }) => `${l.name}:${l.quantity}`).sort()).toEqual([`${rice}:5`, `${tea}:4`])
})

test('purchases: receive a walk-in delivery on credit; it shows under Received → Still owed', async ({ page }) => {
  await admin(page, '/admin/purchases')
  const sup = `Walk-in Supplier ${tag}`
  await page.request.post('/api/v1/admin/suppliers', { data: { name: sup, contact: '', phone: '', address: '', creditDays: 0, openingBalance: '' } })
  const item = `Walk-in Salt ${tag}`
  await freshProduct(page, item)
  await page.getByRole('link', { name: 'Receive delivery' }).click()
  await pick(page, 'Supplier', sup)
  await page.getByLabel('Add a product').fill(item)
  await page.getByRole('button', { name: new RegExp(`^${item}`) }).click()
  await page.getByLabel(`Quantity of ${item}`).fill('3')
  await page.getByLabel(`Cost of ${item}`).fill('70')
  // A scanned code nobody knows becomes the new product's barcode, not its name.
  const code = `99${Date.now()}`.slice(0, 13)
  await page.getByLabel('Add a product').fill(code)
  await page.getByLabel('Add a product').press('Enter')
  await expect(page.getByLabel('New product barcode')).toHaveValue(code)
  await expect(page.getByLabel('New product name')).toHaveValue('')
  await page.getByRole('button', { name: 'Cancel' }).click()
  await page.getByLabel('Pay now').fill('50')
  await page.getByRole('button', { name: 'All', exact: true }).click()
  await expect(page.getByLabel('Pay now')).toHaveValue('210.00')
  await page.getByLabel('Pay now').fill('')
  await page.keyboard.press('F10') // F10 saves
  await toast(page, /on credit/)
  await expect(page.getByText('unpaid')).toBeVisible()
  await page.goto('/admin/purchases?tab=received&unpaid=true')
  await page.getByLabel('Search purchases').fill(sup)
  await expect(page.getByRole('button', { name: new RegExp(sup) }).first()).toBeVisible()

  const sid = (await (await page.request.get(`/api/v1/admin/suppliers?q=${encodeURIComponent(sup)}`)).json()).items[0].id
  const po = await (await page.request.post('/api/v1/admin/purchases', { data: { supplierId: sid, invoiceNo: '', notes: '', expectedDate: '', dueDate: '', discount: '', discountType: 'fixed',
    lines: [{ productId: (await (await page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(item)}`)).json()).items[0].id, name: '', ordered: '', quantity: '1', freeQty: '', costPrice: '70', sellingPrice: '', expiryDate: '', discount: '', discountType: 'fixed' }],
    keepRemainder: false, pay: { amount: '', method: 'cash', source: '' } } })).json()
  await page.goto(`/admin/purchases/${po.id}`)
  await page.getByRole('button', { name: 'Delete' }).click()
  await page.getByRole('button', { name: 'Delete', exact: true }).last().click()
  await toast(page, new RegExp(`Order #${po.id} deleted`))
  await page.waitForURL(/\/admin\/purchases$/)
})

test('reorder: tick items across two searches, pick suppliers, one order per supplier, PO print opens', async ({ page }) => {
  await admin(page, '/admin/reorder')
  const cat = (await (await page.request.get('/api/v1/admin/categories')).json()).find((c: { name: string }) => c.name === 'E2E Goods')
  const mkSup = async (n: string) => (await (await page.request.post('/api/v1/admin/suppliers', { data: { name: n, contact: '', phone: '', address: '', creditDays: 0, openingBalance: '' } })).json()).id as number
  const a = await mkSup(`Reorder Dairy ${tag}`)
  await mkSup(`Reorder Grains ${tag}`)
  for (const [name, sup] of [[`Low Yoghurt ${tag}`, a], [`Low Flour ${tag}`, 0]] as const)
    await page.request.post('/api/v1/admin/products', { data: { name, nameLocal: '', barcode: '', categoryId: cat.id, unitId: 1, costPrice: '40', sellingPrice: '60', wholesalePrice: '', taxRate: '',
      reorderLevel: 5, trackSerial: false, warrantyMonths: 0, supplierId: sup, repriceStock: false, dupSourceName: '' } })
  await page.reload()

  await page.getByLabel('Search products').fill(`Low Yoghurt ${tag}`)
  await expect(page.getByLabel(`Quantity of Low Yoghurt ${tag}`)).toHaveValue('10') // 2 × reorder level, nothing on hand
  await page.getByLabel(`Order Low Yoghurt ${tag}`).check()
  await page.getByLabel('Search products').fill(`Low Flour ${tag}`)
  await page.getByLabel(`Order Low Flour ${tag}`).check()
  await expect(page.getByText('2 items')).toBeVisible()
  await expect(page.getByText('1 need a supplier')).toBeVisible()
  await pick(page, `Supplier for Low Flour ${tag}`, `Reorder Grains ${tag}`)
  await page.getByLabel(`Quantity of Low Flour ${tag}`).fill('25')
  const popup = page.waitForEvent('popup')
  await page.getByRole('button', { name: 'Create orders' }).click()
  await toast(page, /2 purchase orders created/)
  await expect((await popup).getByText(/Purchase order #\d+/).first()).toBeVisible()
  const orders = await (await page.request.get(`/api/v1/admin/purchases?tab=orders&q=${encodeURIComponent(`Reorder Grains ${tag}`)}`)).json()
  expect(orders.total).toBe(1)
  expect(orders.items[0].total).toBe('1000.00') // 25 × 40
})

test('purchase returns: from a delivery with discounts — only its lines, at the cost after discounts, off its bill', async ({ page }) => {
  await admin(page, '/admin/suppliers')
  const sup = `Return Supplier ${tag}`
  const item = `Return Jam ${tag}`
  const sid = (await (await page.request.post('/api/v1/admin/suppliers', { data: { name: sup, contact: '', phone: '', address: '', creditDays: 0, openingBalance: '' } })).json()).id
  const pid = await freshProduct(page, item)
  // 10 × 100 less 10% on the line and 100 off the bill → 800 for 10 = 80 each.
  const grn = (await (await page.request.post('/api/v1/admin/purchases/receive', { data: { supplierId: sid, invoiceNo: `JAM-${tag}`, notes: '', expectedDate: '', dueDate: '', discount: '100', discountType: 'fixed',
    lines: [{ productId: pid, name: '', ordered: '', quantity: '10', freeQty: '', costPrice: '100', sellingPrice: '', expiryDate: '', discount: '10', discountType: 'percent' }], keepRemainder: false, pay: { amount: '', method: 'cash', source: '' } } })).json()).purchaseId
  await page.goto(`/admin/purchases/${grn}`)
  await page.getByRole('link', { name: 'Return goods' }).click()
  await expect(page.getByText(/10 of 10 pcs left from this delivery · Rs\. 80\.00 each/)).toBeVisible()
  await expect(page.getByText('after discounts')).toBeVisible()
  await page.getByLabel(`Return how many ${item}`).fill('11')
  await expect(page.getByText('More than is left from this delivery.')).toBeVisible()
  await page.getByLabel(`Return how many ${item}`).fill('4')
  await page.getByLabel('Reason').fill('Damaged in transit')
  await page.getByRole('button', { name: 'Return Rs. 320.00 of goods' }).click()
  await toast(page, new RegExp(`Returned Rs\\. 320\\.00 of stock to ${sup}`))
  await expect(page.getByText(`JAM-${tag}`)).toBeVisible()
  expect((await (await page.request.get(`/api/v1/admin/suppliers/${sid}`)).json()).owed).toBe('480.00')
  expect((await (await page.request.get(`/api/v1/admin/purchases/${grn}`)).json()).credited).toBe('320.00')

  // From the supplier page: pick the delivery; 6 are left.
  await page.goto(`/admin/suppliers/${sid}`)
  await page.getByRole('button', { name: 'More' }).click()
  await page.getByRole('menuitem', { name: 'Return goods' }).click()
  await pick(page, 'Delivery', `JAM-${tag}`)
  await expect(page.getByText(/6 of 10 pcs left/)).toBeVisible()
  await page.getByRole('button', { name: `More ${item}` }).click() // touch stepper
  await page.getByRole('button', { name: `More ${item}` }).click()
  await expect(page.getByLabel(`Return how many ${item}`)).toHaveValue('2')

  // Old stock — counted in, never on a delivery — goes back at its own cost (40), off the oldest bill.
  const old = `Old Pickle ${tag}`
  const oid = await freshProduct(page, old)
  await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: oid, newQuantity: '5', note: 'opening stock', batchId: 0 } })
  await pick(page, 'Delivery', 'Not from a delivery')
  await page.getByRole('button', { name: 'Add a product' }).click()
  await page.getByLabel('Find a product').fill(old)
  await page.getByRole('button', { name: new RegExp(`^${old}`) }).click()
  await expect(page.getByText(/5 pcs left of old stock · Rs\. 40\.00 each/)).toBeVisible()
  await page.getByRole('button', { name: `More ${old}` }).click()
  await page.getByRole('button', { name: `More ${old}` }).click()
  await page.getByRole('button', { name: 'Return Rs. 80.00 of goods' }).click()
  await toast(page, /Returned Rs\. 80\.00 of stock/)
  expect((await (await page.request.get(`/api/v1/admin/suppliers/${sid}`)).json()).owed).toBe('400.00')
})
