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

const prep: Record<string, (page: Page) => Promise<void>> = {
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
