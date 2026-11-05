import { test, type Page } from '../e2e/fixtures'
import { cashier, code, desk, findProduct, only, out, owner, phone, pickFirst, scan, session, superStore, system } from './demo'

// Screenshots for README + docs/guide, taken from a running Demo Room's
// Karots Super Store (every plugin, a year of trade). `make docs-shots` builds
// nothing: start the demo first (`make demo`, then dist/demo/karots-suite-demo-room).
// DOCS_ONLY=<substring> retakes only the matching shots.
// Output: docs/images/<area>/<name>.jpg — stable names, the guides link to them.
test.skip(!code, 'set DOCS_CODE to the Demo Room hourly code')
test.describe.configure({ mode: 'serial' })

async function shot(page: Page, path: string, name: string, opts: { full?: boolean; prep?: (p: Page) => Promise<void> } = {}) {
  if (only && !name.includes(only)) return
  await page.goto(path)
  await page.waitForLoadState('networkidle')
  await opts.prep?.(page)
  // A full-page shot stitches the page, so the fixed dock would land mid-page.
  if (opts.full) await page.addStyleTag({ content: 'nav[aria-label="Main"] { display: none !important }' })
  await page.waitForTimeout(700) // charts and Motion settle
  await page.screenshot({ path: `${out}/${name}.jpg`, type: 'jpeg', quality: 82, fullPage: !!opts.full })
}

const preps: Record<string, (p: Page) => Promise<void>> = {
  'purchasing/purchases': (p) => p.getByRole('button', { name: 'Received', exact: true }).click(),
  'stock/intake': scan(['4790001002030'], 'Scan or search'),
  'printing/labels': async (p) => {
    await p.getByRole('button', { name: /From a product/ }).click()
    await p.getByRole('dialog').getByRole('textbox').first().fill('Anchor')
    await p.waitForLoadState('networkidle')
    await p.getByRole('dialog').getByRole('button', { name: /Anchor/ }).first().click()
  },
}
const counterPreps: Record<string, (p: Page) => Promise<void>> = {
  returns: pickFirst('Sales'),
  receipts: pickFirst('Sales'),
  credit: pickFirst('Customers'),
  suppliers: pickFirst('Suppliers'),
  warranty: pickFirst('Units'),
  convert: pickFirst('ul'),
  'write-off': findProduct('Milo'),
  labels: findProduct('Anchor'),
}

test('demo picker, then the Super Store comes up', async ({ browser }) => {
  const page = await session(browser, null)
  const st = await (await page.request.get('/__demo/state')).json()
  if (st.phase === 'picker') await shot(page, '/', 'demo/picker')
  await superStore(page)
  await page.context().close()
})

test('login', async ({ browser }) => {
  for (const mode of ['light', 'dark'] as const) {
    const page = await session(browser, null, desk, mode)
    await shot(page, '/login', `start/login-${mode}`)
    await page.context().close()
  }
})

const admin: [string, string][] = [
  ['/admin', 'admin/dashboard'],
  ['/admin/sales', 'admin/sales'],
  ['/admin/products', 'stock/products'],
  ['/admin/categories', 'stock/categories'],
  ['/admin/stock', 'stock/stock'],
  ['/admin/stock/take', 'stock/stock-take'],
  ['/admin/stock/batches', 'stock/batches'],
  ['/admin/inventory/intake', 'stock/intake'],
  ['/admin/reorder', 'purchasing/reorder'],
  ['/admin/purchases', 'purchasing/purchases'],
  ['/admin/suppliers', 'purchasing/suppliers'],
  ['/admin/purchase-returns', 'purchasing/returns'],
  ['/admin/cashflow', 'money/cashflow'],
  ['/admin/finance', 'money/finance'],
  ['/admin/lockers', 'money/lockers'],
  ['/admin/expenses', 'money/expenses'],
  ['/admin/cheques', 'money/cheques'],
  ['/admin/tills', 'money/tills'],
  ['/admin/receipts', 'money/receipts'],
  ['/admin/reports', 'reports/index'],
  ['/admin/reports/sales', 'reports/sales'],
  ['/admin/reports/top-products', 'reports/top-products'],
  ['/admin/reports/peak-hours', 'reports/peak-hours'],
  ['/admin/reports/profit-by-category', 'reports/profit-by-category'],
  ['/admin/customers', 'people/customers'],
  ['/admin/users', 'people/users'],
  ['/admin/activity', 'people/activity'],
  ['/admin/labels', 'printing/labels'],
  ['/admin/settings', 'admin/settings'],
  ['/admin/data', 'install/data'],
]
const plugins = ['recharge', 'pharmacy', 'repairs', 'documents', 'alternatives', 'clearance', 'aicatalog', 'productplus', 'combos', 'requests']

test('admin, desktop', async ({ browser }) => {
  const page = await session(browser, owner)
  for (const [path, name] of admin) await shot(page, path, name, { prep: preps[name] })
  for (const key of plugins) await shot(page, `/admin/x/${key}`, `plugins/${key}`)
  await shot(page, '/admin/x/aicatalog/jobs', 'plugins/aicatalog-jobs')
  await page.context().close()
})

test('admin, phone', async ({ browser }) => {
  const page = await session(browser, owner, phone)
  for (const [path, name] of admin.filter(([, n]) => /dashboard|products|cashflow|reports\/sales|stock\/stock$/.test(n)))
    await shot(page, path, `phone/${name.split('/')[1]}`)
  await page.context().close()
})

test('dark mode', async ({ browser }) => {
  const page = await session(browser, owner, desk, 'dark')
  await shot(page, '/admin', 'admin/dashboard-dark')
  await page.context().close()
})

const counter = ['returns', 'credit', 'receipts', 'low-stock', 'labels', 'suppliers', 'expenses', 'write-off', 'convert', 'warranty']
const cart = scan(['4790001007493', '4790001002030', '4790001011766'], 'Scan a barcode or search products')

test('till', async ({ browser }) => {
  for (const mode of ['light', 'dark'] as const) {
    const page = await session(browser, cashier, desk, mode)
    await shot(page, '/till', `till/sell-${mode}`, { prep: cart })
    if (mode === 'light') for (const tab of counter) await shot(page, `/till/${tab}`, `till/${tab}`, { prep: counterPreps[tab] })
    await page.context().close()
  }
})

// Send to cashier: a sales person's cart, the ticket on a cashier's till, and the close that waits for it.
test('send to cashier', async ({ browser }) => {
  if (only && !'till/send-to-cashier till/tickets till/close-tickets'.includes(only)) return
  const taker = await session(browser, { phone: '0771000003', pin: '3333' })
  await taker.request.post('/__demo/cheat/open-till', { data: {} })
  await taker.goto('/till')
  await taker.waitForLoadState('networkidle')
  const seller = await session(browser, { phone: '0771000005', pin: '5555' })
  await shot(seller, '/till', 'till/send-to-cashier', { prep: cart })
  await seller.getByRole('button', { name: /^Send to Pradeep/ }).click()
  const tickets = taker.getByRole('group', { name: 'Tickets' })
  await tickets.waitFor({ timeout: 15_000 })
  await taker.waitForTimeout(700)
  await taker.screenshot({ path: `${out}/till/tickets.jpg`, type: 'jpeg', quality: 82 })
  await taker.getByRole('button', { name: 'Close till' }).click()
  await taker.waitForTimeout(700)
  await taker.screenshot({ path: `${out}/till/close-tickets.jpg`, type: 'jpeg', quality: 82 })
  await seller.context().close()
  await taker.context().close()
})

// A new batch with a new barcode: the question on a delivery, then the link waiting on Items to review.
test('unknown scan and barcode review', async ({ browser }) => {
  const names = 'purchasing/unknown-scan stock/review-barcode'
  if (only && !names.includes(only)) return
  const page = await session(browser, owner)
  await page.goto('/admin/purchases/new')
  await page.getByLabel('Add a product').fill('4790999000017')
  await page.getByLabel('Add a product').press('Enter')
  await page.getByRole('button', { name: "It's a new product" }).waitFor()
  await page.waitForTimeout(500)
  await page.screenshot({ path: `${out}/purchasing/unknown-scan.jpg`, type: 'jpeg', quality: 82 })

  const till = await session(browser, { phone: '0771000003', pin: '3333' })
  const p = await (await till.request.get('/api/v1/till/products/barcode/4790001002030')).json()
  const r = await till.request.post(`/api/v1/till/products/${p.id}/barcodes`, { data: { barcode: '4790999000017', source: 'receive' } })
  if (!r.ok()) throw new Error(`link: ${r.status()}`)
  await page.goto('/admin/products/review')
  await page.getByText('Barcode 4790999000017 linked here by').locator('visible=true').waitFor()
  await page.waitForTimeout(500)
  await page.screenshot({ path: `${out}/stock/review-barcode.jpg`, type: 'jpeg', quality: 82 })
  await till.context().close()
  await page.context().close()
})

// A cashier PC's print helper has called in, waiting for the admin's OK.
test('cashier PC printers', async ({ browser }) => {
  if (only && !'printing/cashier-pc-printers'.includes(only)) return
  const page = await session(browser, owner)
  const r = await page.request.post('/api/v1/print-agent/poll', { headers: { 'X-Ks-Print-Agent': 'docs-shots-print-helper-token-000000000' }, data: { name: 'Counter laptop', printers: ['XP-80T', 'XP-365B'] } })
  if (!r.ok()) throw new Error(`helper: ${r.status()}`)
  await page.goto('/admin/settings')
  const card = page.getByRole('region', { name: "Cashier PCs' printers" })
  await card.getByLabel('Counter laptop Receipt printer').selectOption('XP-80T')
  await card.getByLabel('Counter laptop Label printer').selectOption('XP-365B')
  await card.screenshot({ path: `${out}/printing/cashier-pc-printers.jpg`, type: 'jpeg', quality: 82 })
  const rows: { id: number }[] = await (await page.request.get('/api/v1/admin/print-machines')).json()
  for (const m of rows) await page.request.delete(`/api/v1/admin/print-machines/${m.id}`)
  await page.context().close()
})

const palettes = ['studio', 'night', 'bazaar', 'lagoon', 'catppuccin', 'nord', 'dracula', 'mango', 'synthwave', 'gruvbox']

test('appearance and palettes', async ({ browser }) => {
  const light = await session(browser, system)
  const dark = await session(browser, system, desk, 'dark')
  await shot(light, '/admin/system/appearance', 'appearance/appearance', { full: true })
  // Shoot one Appearance section, opened on its own (the page remembers open sections).
  const section = async (name: string, file: string) => {
    if (only && !file.includes(only)) return
    await light.goto('/admin/system/appearance')
    await light.waitForLoadState('networkidle')
    await light.evaluate((n) => document.querySelectorAll('details').forEach((d) => {
      d.open = !!d.querySelector('summary')?.textContent?.startsWith(n)
    }), name)
    await light.addStyleTag({ content: 'nav[aria-label="Main"] { display: none !important }' })
    await light.waitForTimeout(1000)
    await light.locator('details[open]').screenshot({ path: `${out}/${file}.jpg`, type: 'jpeg', quality: 82 })
  }
  await section('Receipt style', 'printing/receipt-styles')
  await section('Label styles', 'printing/label-styles')
  const cur = await (await light.request.get('/api/v1/system/appearance')).json()
  try {
    for (const palette of palettes) {
      await light.request.put('/api/v1/system/appearance', { data: { ...cur, palette } })
      await shot(light, '/admin', `appearance/palette-${palette}-light`)
      await shot(dark, '/admin', `appearance/palette-${palette}-dark`)
    }
  } finally {
    await light.request.put('/api/v1/system/appearance', { data: cur })
  }
  await light.context().close()
  await dark.context().close()
})

// The Pocket Cashier app on a phone: paired, approved by the owner, a cashier selling.
const app = 'pocketcashier' // an app (plugin.json kind "app"): its own phone shot, not the plugins loop

test('pocket cashier, phone', async ({ browser }) => {
  if (only && !`plugins/${app}`.includes(only)) return
  const admin = await session(browser, owner)
  const users: { phone: string; name: string }[] = await (await admin.request.get('/api/v1/admin/users')).json()
  const name = users.find((u) => u.phone === cashier.phone)!.name
  const page = await session(browser, null, phone, 'light', { isMobile: true, hasTouch: true })
  await page.goto(`/x/${app}/`)
  await page.getByLabel('This phone’s name').fill('Counter phone')
  await page.getByRole('button', { name: 'Pair this phone' }).click()
  const pairCode = (await page.getByLabel('Pairing code').textContent())!.trim()
  await admin.request.post(`/api/v1/x/${app}/phones/approve`, { data: { code: pairCode } })
  await page.getByRole('radiogroup', { name: 'Who are you?' }).getByRole('radio', { name: name.split(' ')[0] }).click()
  for (const k of cashier.pin) await page.getByRole('button', { name: k, exact: true }).click()
  await page.getByRole('button', { name: 'Sign in' }).click()
  const products = page.getByRole('list', { name: 'Products' })
  // Whether a product asks which price or pack depends on the year's stock: take the first.
  const tap = async (n: number) => {
    await products.getByRole('button').nth(n).click()
    await page.waitForTimeout(400)
    const sheet = page.getByRole('dialog')
    if (await sheet.isVisible()) {
      await sheet.getByRole('button').first().click()
      await sheet.waitFor({ state: 'hidden' })
    }
  }
  await tap(0)
  await tap(2)
  await tap(2)
  await page.waitForTimeout(700)
  await page.screenshot({ path: `${out}/plugins/${app}.jpg`, type: 'jpeg', quality: 82 })
  await page.context().close()
  await admin.context().close()
})
