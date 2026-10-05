import { test, type Page } from './fixtures'

// UI-gate screenshots for admin pages (not a behaviour test). Needs `make e2e-seed`.
//   SHOTS=m2-1 SHOTS_PATH=/admin/units bunx playwright test e2e/admin-shots.spec.ts
// SHOTS_PREP (optional) names a prep step below run after the page loads.
// Output: web/shots/<gate>/<name>-<palette>-<mode>-<width>.png
const gate = process.env.SHOTS
const path = process.env.SHOTS_PATH ?? '/admin'
const prepKey = process.env.SHOTS_PREP ?? ''
const name = prepKey || path.replace(/^\/admin\/?/, '').replace(/\//g, '-') || 'home'
test.skip(!gate, 'set SHOTS=<gate> to take UI-gate screenshots')
test.describe.configure({ mode: 'serial' })

async function login(page: Page, phone: string, pin: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill(pin)
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(admin|till)/)
}

/** A supplier with history for the M3 shots: old debt, two deliveries, a payment. */
async function supplierWithHistory(page: Page): Promise<number> {
  const post = async (url: string, data: unknown) => (await page.request.post(url, { data })).json()
  const mk = (name: string, opening: string, phone = '', contact = '', days = 0) =>
    post('/api/v1/admin/suppliers', { name, contact, phone, address: 'Pettah, Colombo 11', creditDays: days, openingBalance: opening })
  const n = Date.now().toString(36).slice(-3)
  const s = await mk(`Ceylon Traders ${n}`, '2500', '0112345678', 'Nimal', 30)
  await mk(`Kandy Dairy ${n}`, '0', '0812233445', 'Sunil', 14)
  await mk(`Galle Spices ${n}`, '800', '0912222333')
  const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
  for (const [qty, inv] of [['24', 'CT-1041'], ['12', 'CT-1077']])
    await post(`/api/v1/till/suppliers/${s.id}/receive`, { invoiceNo: inv, discount: '', discountType: 'fixed', notes: '', keepRemainder: false, deferDrawerKick: true,
      pay: { amount: '', method: 'cash', lockerId: 0 }, lines: [{ productId: milk.id, name: '', ordered: '', quantity: qty, freeQty: '', costPrice: '85', sellingPrice: '', expiryDate: '', discount: '', discountType: 'fixed' }] })
  await post(`/api/v1/admin/suppliers/${s.id}/payment`, { amount: '1500', mode: 'old', allocations: [], advance: '', oldDebt: '', method: 'online', reference: 'BOC 88213', note: '', source: '' })
  return s.id
}

/** A credit customer with history: old debt, a credit sale's worth of activity, a payment. */
async function customerWithHistory(page: Page): Promise<number> {
  const post = async (url: string, data: unknown) => (await page.request.post(url, { data })).json()
  const n = String(Date.now()).slice(-7)
  const mk = (name: string, phone: string, limit: string, opening: string) => post('/api/v1/admin/customers', { name, phone, address: 'Kandy Road, Peradeniya', creditLimit: limit, openingBalance: opening })
  const c = await mk('Kamala Perera', `071${n}`, '5000', '3200')
  await mk('Sunil Traders', `072${n}`, '2000', '2600')
  await mk('Ruwan Silva', `075${n}`, '0', '0')
  await post(`/api/v1/admin/customers/${c.id}/payment`, { amount: '1200', method: 'online', reference: 'TXN4471', note: '', source: '', oldDebtFirst: false })
  return c.id
}

/** Two orders and a part-received delivery from the supplier-with-history. */
async function purchasesWithHistory(page: Page): Promise<{ order: number; received: number }> {
  const sup = await supplierWithHistory(page)
  const post = async (url: string, data: unknown) => (await page.request.post(url, { data })).json()
  const byCode = async (c: string) => (await (await page.request.get(`/api/v1/till/products/barcode/${c}`)).json()).id as number
  const milk = await byCode('4790000000011')
  const soap = await byCode('4790000000028')
  const l = (productId: number, quantity: string, costPrice: string, ordered = '') => ({ productId, name: '', ordered, quantity, freeQty: '', costPrice, sellingPrice: '', expiryDate: '', discount: '', discountType: 'fixed' })
  const order = (await post('/api/v1/admin/purchases', { supplierId: sup, invoiceNo: '', notes: 'Deliver before noon', expectedDate: new Date(Date.now() + 3 * 864e5).toISOString().slice(0, 10), dueDate: '', discount: '5', discountType: 'percent', lines: [l(milk, '24', '85'), l(soap, '12', '50')], keepRemainder: false, pay: { amount: '', method: 'cash', source: '' } })).id
  const second = (await post('/api/v1/admin/purchases', { supplierId: sup, invoiceNo: '', notes: '', expectedDate: '', dueDate: '', discount: '', discountType: 'fixed', lines: [l(milk, '48', '85'), l(soap, '6', '52')], keepRemainder: false, pay: { amount: '', method: 'cash', source: '' } })).id
  await post(`/api/v1/admin/purchases/${second}/receive`, { supplierId: sup, invoiceNo: 'CT-1102', notes: '', expectedDate: '', dueDate: new Date(Date.now() + 30 * 864e5).toISOString().slice(0, 10), discount: '', discountType: 'fixed', lines: [l(milk, '40', '88', '48'), l(soap, '6', '52', '6')], keepRemainder: true, pay: { amount: '1000', method: 'online', source: '' } })
  return { order, received: second }
}

/** Four lockers (made once), the safe as the till default, and some money through
 * the safe: a customer paying in, a supplier paid out. Returns the safe's id. */
async function lockersWithHistory(page: Page): Promise<number> {
  const post = async (url: string, data: unknown) => (await page.request.post(url, { data })).json()
  const have = (await (await page.request.get('/api/v1/admin/lockers')).json()).items as { id: number; name: string }[]
  const find = (n: string) => have.find((l) => l.name === n)?.id
  for (const [name, kind, openingBalance, overdraft, cashierAccess] of [['Shop safe', 'safe', '48250', false, true], ['BOC current account', 'bank', '312500', false, false],
    ["Owner's pocket", 'pocket', '0', true, false], ['Card machine', 'card', '18640', false, true]] as const)
    if (!find(name)) have.push(await post('/api/v1/admin/lockers', { name, kind, openingBalance, overdraft, cashierAccess }))
  const safe = find('Shop safe')!
  await page.request.put('/api/v1/admin/settings/till-cash', { data: { defaultLockerId: safe, allowUntracked: false } })
  if ((await (await page.request.get(`/api/v1/admin/lockers/${safe}/ledger?`)).json()).length < 3) {
    await post(`/api/v1/admin/customers/${await customerWithHistory(page)}/payment`, { amount: '1800', method: 'cash', reference: '', note: 'Paid at the shop', source: `locker:${safe}`, oldDebtFirst: false })
    await post(`/api/v1/admin/suppliers/${await supplierWithHistory(page)}/payment`, { amount: '1000', mode: 'old', allocations: [], advance: '', oldDebt: '', method: 'cash', reference: '', note: 'Old balance', source: `locker:${safe}` })
  }
  return safe
}

/** Choose the option of a labelled <select> whose text starts with `text`. */
async function pick(page: Page, label: string, text: string) {
  const sel = page.getByLabel(label, { exact: true })
  await sel.selectOption((await sel.locator('option', { hasText: text }).first().getAttribute('value'))!)
}

/** A month of expenses (made once): some paid from the safe, some from outside. */
async function expensesWithHistory(page: Page) {
  const safe = await lockersWithHistory(page)
  if (((await (await page.request.get('/api/v1/admin/expenses?preset=this-month&q=CEB')).json()) as unknown[]).length) return
  const d = (n: number) => new Date(Date.now() - n * 864e5).toLocaleDateString('en-CA')
  for (const [category, amount, description, n, source] of [['Rent', '45000', 'October shop rent', 4, ''], ['Electricity', '8420', 'CEB bill', 3, `locker:${safe}`],
    ['Transport', '1200', 'Tuk to Pettah wholesale', 2, `locker:${safe}`], ['Salary', '15000', 'Nimali — half month', 1, ''], ['Water', '1350', 'NWSDB', 0, `locker:${safe}`]] as const)
    await page.request.post('/api/v1/admin/expenses', { data: { category, amount, description, date: d(n), serviceId: 0, source } })
}

const prep: Record<string, (page: Page) => Promise<void>> = {
  expenses: async (page) => {
    await expensesWithHistory(page)
    await page.reload()
    await page.waitForLoadState('networkidle')
  },
  'expense-new': async (page) => {
    await prep.expenses(page)
    await page.getByRole('button', { name: '↑ Record expense' }).click()
    await page.getByRole('button', { name: 'Transport', exact: true }).click()
    await page.getByLabel('Amount', { exact: true }).fill('650')
    await pick(page, 'Paid from', 'Shop safe')
    await page.getByLabel('Note').fill('Courier to Kandy')
  },
  'expense-void': async (page) => {
    await prep.expenses(page)
    await page.getByRole('button', { name: /^Electricity.*CEB bill/ }).locator('visible=true').first().click()
    await page.getByRole('button', { name: 'Void…' }).click()
    await page.waitForLoadState('networkidle')
  },
  lockers: async (page) => {
    await lockersWithHistory(page)
    await page.reload()
    await page.waitForLoadState('networkidle')
  },
  'till-cash': async (page) => {
    await prep.lockers(page)
    await page.getByRole('region', { name: 'At the till' }).scrollIntoViewIfNeeded()
  },
  locker: async (page) => {
    await page.goto(`/admin/lockers/${await lockersWithHistory(page)}`)
    await page.waitForLoadState('networkidle')
  },
  'locker-transfer': async (page) => {
    await prep.lockers(page)
    await page.getByRole('button', { name: 'Move money' }).click()
    await pick(page, 'From', 'Shop safe')
    await pick(page, 'To', 'BOC current')
    await page.getByLabel('Amount', { exact: true }).fill('25000')
    await page.getByLabel('Note').fill("Banked the week's takings")
  },
  'locker-adjust': async (page) => {
    await page.goto(`/admin/lockers/${await lockersWithHistory(page)}`)
    await page.getByRole('button', { name: 'Adjust' }).click()
    await page.getByRole('radio', { name: /Count is lower/ }).click()
    await page.getByLabel('Amount', { exact: true }).fill('500')
    await page.getByLabel('Note').fill('Counted the safe — one note short')
  },
  'settle-card': async (page) => {
    await prep.lockers(page)
    await page.getByRole('button', { name: 'Settle card' }).click()
    await pick(page, 'Card machine', 'Card machine')
    await pick(page, 'Into bank', 'BOC current')
    await page.getByLabel('Service fee').fill('372.80')
  },
  'locker-form': async (page) => {
    await lockersWithHistory(page)
    await page.reload()
    await page.getByRole('button', { name: 'New locker' }).click()
    await page.getByRole('textbox', { name: 'Name', exact: true }).fill('Sampath savings')
    await page.getByRole('radio', { name: 'Bank account' }).click()
    await page.getByLabel('Opening balance').fill('150000')
  },
  'return-entry': async (page) => {
    await page.goto(`/admin/purchase-returns/new?purchase=${(await purchasesWithHistory(page)).received}`)
    await page.waitForLoadState('networkidle')
    await page.getByLabel('Return how many E2E Milk').fill('6')
    await page.getByLabel('Reason').fill('Short-dated')
  },
  'return-old': async (page) => {
    const sup = await supplierWithHistory(page)
    await page.goto(`/admin/purchase-returns/new?supplier=${sup}`)
    await page.waitForLoadState('networkidle')
    const sel = page.getByLabel('Delivery')
    await sel.selectOption((await sel.locator('option', { hasText: 'Not from a delivery' }).getAttribute('value'))!)
    await page.getByRole('button', { name: 'Add a product' }).click()
    await page.getByLabel('Find a product').fill('Lot Soap')
    await page.getByRole('button', { name: /^E2E Lot Soap/ }).first().click()
    await page.getByRole('button', { name: 'More E2E Lot Soap' }).click()
  },
  returns: async (page) => {
    const sup = await supplierWithHistory(page)
    const milk = (await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()).id
    const grn = (await (await page.request.get(`/api/v1/admin/purchases?tab=received&supplier=${sup}`)).json()).items[0].id
    const lot = (await (await page.request.get(`/api/v1/admin/purchases/${grn}/returnable`)).json()).lines.find((l: { productId: number }) => l.productId === milk)
    await page.request.post('/api/v1/admin/purchase-returns', { data: { supplierId: sup, purchaseId: grn, reference: 'DN-204', reason: 'Damaged in transit', lines: [{ productId: milk, quantity: '3', costPrice: '', batchId: lot.batchId }] } })
    await page.reload()
    await page.waitForLoadState('networkidle')
  },
  users: async (page) => {
    const n = String(Date.now()).slice(-6)
    for (const [name, role, extra] of [['Nimali Fernando', 'cashier', { canHandleSuppliers: true, canSeeCost: true }], ['Kasun Jayasinghe', 'manager', {}], ['Dilani Perera', 'cashier', { canManageCredit: true, receiptPrinter: 'tcp://192.168.1.51:9100' }]] as const)
      await page.request.post('/api/v1/admin/users', { data: { name, phone: `07${role.length}${n}${name.length % 10}`, role, pin: '4321', receiptPrinter: '', canHandleSuppliers: false, canManageCredit: false, canSeeCost: false, ...extra } })
    await page.reload()
    await page.waitForLoadState('networkidle')
  },
  'user-form': async (page) => {
    await page.getByRole('button', { name: 'New user' }).click()
    await page.getByLabel('Name').fill('Tharindu Silva')
    await page.getByLabel('Phone').fill('0771234567')
    await page.getByLabel('PIN').fill('4321')
  },
  reorder: async (page) => {
    const cat = (await (await page.request.get('/api/v1/admin/categories')).json()).find((c: { name: string }) => c.name === 'E2E Goods')
    const n = Date.now().toString(36).slice(-3)
    for (const [name, lvl] of [[`Basmati 5kg ${n}`, 8], [`Coconut oil 1L ${n}`, 12], [`Dhal 1kg ${n}`, 6]] as const)
      await page.request.post('/api/v1/admin/products', { data: { name, nameLocal: '', barcode: '', categoryId: cat.id, unitId: 1, costPrice: '420', sellingPrice: '520', wholesalePrice: '', taxRate: '',
        reorderLevel: lvl, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '' } })
    await page.reload()
    await page.getByLabel('Search products').fill(n)
    await page.getByLabel(`Order Basmati 5kg ${n}`).check()
    await page.getByLabel(`Order Dhal 1kg ${n}`).check()
  },
  purchases: async (page) => {
    await purchasesWithHistory(page)
    await page.reload()
    await page.waitForLoadState('networkidle')
  },
  'purchases-received': async (page) => {
    await purchasesWithHistory(page)
    await page.goto('/admin/purchases?tab=received')
    await page.waitForLoadState('networkidle')
  },
  purchase: async (page) => {
    await page.goto(`/admin/purchases/${(await purchasesWithHistory(page)).received}`)
    await page.waitForLoadState('networkidle')
  },
  'purchase-receive': async (page) => {
    await page.goto(`/admin/purchases/${(await purchasesWithHistory(page)).order}/receive`)
    await page.waitForLoadState('networkidle')
    await page.getByLabel('Quantity of E2E Milk').fill('20')
    await page.getByLabel('Cost of E2E Milk').fill('120')
  },
  po: async (page) => {
    await page.goto(`/print/po?ids=${(await purchasesWithHistory(page)).order}`)
    await page.waitForLoadState('networkidle')
  },
  grn: async (page) => {
    await page.goto(`/print/grn/${(await purchasesWithHistory(page)).received}`)
    await page.waitForLoadState('networkidle')
  },
  customers: async (page) => {
    await customerWithHistory(page)
    await page.reload()
    await page.waitForLoadState('networkidle')
  },
  customer: async (page) => {
    await page.goto(`/admin/customers/${await customerWithHistory(page)}`)
    await page.waitForLoadState('networkidle')
  },
  'customer-pay': async (page) => {
    await prep.customer(page)
    await page.getByRole('button', { name: '↓ Collect' }).click()
  },
  suppliers: async (page) => {
    await supplierWithHistory(page)
    await page.reload()
    await page.waitForLoadState('networkidle')
  },
  supplier: async (page) => {
    await page.goto(`/admin/suppliers/${await supplierWithHistory(page)}`)
    await page.waitForLoadState('networkidle')
  },
  'supplier-pay': async (page) => {
    await prep.supplier(page)
    await page.getByRole('button', { name: '↑ Pay' }).click()
    await page.getByRole('button', { name: 'Split exactly' }).click()
  },
  'supplier-opening': async (page) => {
    await prep.supplier(page)
    await page.getByRole('button', { name: 'More' }).click()
    await page.getByRole('menuitem', { name: 'Old balance' }).click()
  },
  statement: async (page) => {
    await page.goto(`/print/statement/supplier/${await supplierWithHistory(page)}`)
    await page.waitForLoadState('networkidle')
  },
  'unit-form': async (page) => {
    await page.getByRole('button', { name: /New unit/ }).click()
    await page.getByLabel('Name').fill('Kilogram')
  },
  categories: async (page) => {
    const mk = async (name: string, parentId = 0) =>
      (await (await page.request.post('/api/v1/admin/categories/quick', { data: { name, parentId } })).json()).id as number
    const food = await mk('Groceries')
    const rice = await mk('Rice & grains', food)
    await mk('Basmati', rice)
    await mk('Red rice', rice)
    await mk('Spices', food)
    const st = await mk('Stationery')
    await mk('Pens', st)
    await page.reload()
    await page.waitForLoadState('networkidle')
  },
  'category-form': async (page) => {
    await prep.categories(page)
    await page.getByRole('button', { name: /^Rice & grains \d/ }).click()
    await page.getByRole('button', { name: 'Parent category' }).click()
  },
  'product-form': async (page) => {
    await page.getByLabel('Search products').fill('E2E Milk')
    await page.getByRole('button', { name: /^E2E Milk\b/ }).first().click()
    await page.getByLabel('Selling').fill('135')
  },
  review: async (page) => {
    const have = await (await page.request.get('/api/v1/admin/products/review')).json()
    if (have.length === 0) for (const [name, price] of [['Coconut scraper', '450'], ['Blue gift bag', '60']])
      await page.request.post('/api/v1/till/quick-item', { data: { name, price, qty: '1', barcode: '', unitId: 0 } })
    await page.reload()
    await page.waitForLoadState('networkidle')
  },
  services: async (page) => {
    const have = await (await page.request.get('/api/v1/admin/services')).json()
    if (!have.some((s: { name: string }) => s.name === 'Milk tea')) {
      const cats = await (await page.request.get('/api/v1/admin/categories')).json()
      const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
      const soap = await (await page.request.get('/api/v1/till/products/barcode/4790000000028')).json()
      const svc = await (await page.request.post('/api/v1/admin/services', { data: { name: 'Milk tea', categoryId: cats[0].id, unitId: 1, sellingPrice: '60' } })).json()
      await page.request.post(`/api/v1/admin/products/${svc.id}/recipe`, { data: {
        components: [{ productId: milk.id, amount: '0.25', mode: 'per_unit', whole: false }, { productId: soap.id, amount: '40', mode: 'yield', whole: false }],
        costs: [{ label: 'Gas', amount: '3' }] } })
      await page.request.post('/api/v1/admin/services', { data: { name: 'Photocopy A4', categoryId: cats[0].id, unitId: 1, sellingPrice: '10' } })
      await page.reload()
      await page.waitForLoadState('networkidle')
    }
  },
  recipe: async (page) => {
    await prep.services(page)
    await page.getByRole('button', { name: /^Milk tea/ }).first().click()
    await page.getByLabel('Ingredients').waitFor()
  },
  groups: async (page) => {
    const have = await (await page.request.get('/api/v1/admin/groups')).json()
    if (!have.some((g: { name: string }) => g.name === 'Dairy')) {
      const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
      const yog = await (await page.request.get('/api/v1/till/products/barcode/4790000000035')).json()
      const mk = async (name: string, emoji: string, parentId = 0) => (await (await page.request.post('/api/v1/admin/groups', { data: { name, emoji, parentId } })).json()).id
      const dairy = await mk('Dairy', '🥛')
      const fresh = await mk('Fresh milk', '🍼', dairy)
      await mk('Snacks', '🍪')
      await page.request.post(`/api/v1/admin/groups/${fresh}/items`, { data: { productId: milk.id, emoji: '🥛' } })
      await page.request.post(`/api/v1/admin/groups/${fresh}/items`, { data: { productId: yog.id, emoji: '🍶' } })
      await page.reload()
      await page.waitForLoadState('networkidle')
    }
    await page.getByRole('button', { name: /^🍼 Fresh milk/ }).click()
  },
  emoji: async (page) => {
    await prep.groups(page)
    await page.getByRole('button', { name: 'Emoji for E2E Milk' }).click()
  },
  'emoji-search': async (page) => {
    await prep.emoji(page)
    await page.getByLabel('Search emoji').fill('milk')
  },
  lots: async (page) => {
    await page.getByLabel('Search products').fill('Lot Soap')
    await page.getByRole('button', { name: /^E2E Lot Soap/ }).first().click()
    await page.getByLabel('Batches', { exact: true }).waitFor()
  },
  'batch-count': async (page) => {
    await prep.lots(page)
    await page.getByLabel('Batch 1').getByRole('button', { name: 'Set count' }).click()
    await page.getByLabel('Batch 2').getByRole('button', { name: 'Remove batch' }).click()
  },
  adjust: async (page) => {
    await page.getByLabel('Search products').fill('Lot Soap')
    await page.getByRole('button', { name: 'Adjust', exact: true }).first().click()
    await page.getByLabel('Actually there').fill('7')
    await page.getByRole('button', { name: 'Missing' }).click()
  },
  moves: async (page) => {
    const soap = await (await page.request.get('/api/v1/till/products/barcode/4790000000028')).json()
    const have = await (await page.request.get(`/api/v1/admin/stock/movements?type=adjust&product=${soap.id}`)).json()
    if (have.total === 0) {
      await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: soap.id, newQuantity: '9', note: 'Count correction', batchId: 0 } })
      await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: soap.id, newQuantity: '12', note: 'Found extra', batchId: 0 } })
    }
    await page.goto('/admin/stock?view=moves')
    await page.waitForLoadState('networkidle')
  },
  take: async (page) => {
    await page.getByLabel('Counted E2E Lot Soap').fill('8')
    await page.getByLabel('Counted E2E Lot Soap').press('Enter')
    await page.keyboard.type('480')
    await page.getByLabel('Cost of E2E Milk', { exact: true }).fill('82')
  },
  'intake-restock': async (page) => {
    await page.getByLabel('Scan or search').fill('4790000000011')
    await page.getByLabel('Scan or search').press('Enter')
    await page.getByLabel('Quantity to add').fill('24')
    await page.getByLabel('Selling').fill('110')
  },
  'intake-new': async (page) => {
    await page.getByLabel('Scan or search').fill('Kithul treacle 750ml')
    await page.getByLabel('Scan or search').press('Enter')
    await page.getByLabel('Cost').fill('900')
    await page.getByLabel('Markup').fill('1.3')
    await page.getByLabel('Opening quantity').fill('6')
    await page.getByRole('button', { name: 'Generate' }).click()
  },
  'intake-session': async (page) => {
    await prep['intake-restock'](page)
    await page.getByLabel('Print labels on save').uncheck()
    await page.getByRole('button', { name: 'Add stock', exact: true }).click()
    await page.getByLabel('Scan or search').fill('4790000000028')
    await page.getByLabel('Scan or search').press('Enter')
    await page.getByLabel('Quantity to add').fill('10')
    await page.getByRole('button', { name: 'Add stock', exact: true }).click()
    await page.getByText('Nothing yet').waitFor({ state: 'detached' })
  },
  'conv-run': async (page) => {
    await page.getByRole('button', { name: 'Run', exact: true }).first().click()
    await page.getByLabel('Packs to break').fill('2')
  },
  'conv-runs': async (page) => {
    await page.getByRole('button', { name: 'Run history' }).click()
    await page.waitForLoadState('networkidle')
  },
  'labels-product': async (page) => {
    await page.getByRole('button', { name: /From a product/ }).click()
    await page.getByLabel('Find a product').fill('Lot Soap')
    await page.getByRole('button', { name: /^E2E Lot Soap/ }).click()
    await page.getByLabel('Label preview').waitFor()
  },
  'label-sheet': async (page) => {
    await page.goto('/print/labels?code=4790000000011&top=E2E%20Milk&bottom=Rs.%20100.00&count=12&size=50x25')
    await page.getByLabel('Label 12').waitFor()
  },
  'write-off': async (page) => {
    await page.getByRole('button', { name: 'Record write-off' }).click()
    await page.getByRole('button', { name: 'Product', exact: true }).click()
    await page.getByLabel('Find a product').fill('Lot Soap')
    await page.getByRole('button', { name: /^E2E Lot Soap/ }).click()
    await page.getByLabel('Quantity').fill('2')
  },
  shelf: async (page) => {
    await page.getByRole('button', { name: 'Setup' }).first().click()
  },
}

for (const palette of ['studio', 'night']) {
  for (const mode of ['light', 'dark']) {
    for (const width of [1366, 360]) {
      test(`${palette} ${mode} ${width}`, async ({ page }) => {
        await page.setViewportSize({ width, height: width < 768 ? 780 : 768 })
        await login(page, '0000000001', process.env.POS_SYSTEM_PIN ?? '2273')
        const cur = await (await page.request.get('/api/v1/system/appearance')).json()
        await page.request.put('/api/v1/system/appearance', { data: { ...cur, palette } })
        await page.goto('about:blank')
        await page.context().clearCookies()
        await page.addInitScript((m) => localStorage.setItem('ks-mode', m), mode)
        await login(page, '0770000001', '1234')
        await page.goto(path)
        await page.waitForLoadState('networkidle')
        await prep[prepKey]?.(page)
        await page.waitForTimeout(300)
        await page.screenshot({ path: `shots/${gate}/${name}-${palette}-${mode}-${width}.png` })
      })
    }
  }
}
