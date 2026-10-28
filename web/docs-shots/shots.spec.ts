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
