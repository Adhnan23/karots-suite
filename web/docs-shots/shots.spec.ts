import type { Browser } from '@playwright/test'
import { test, type Page } from '../e2e/fixtures'

// Screenshots for README + docs/guide, taken from a running Demo Room's
// Karots Super Store (every plugin, a year of trade). `make docs-shots` builds
// nothing: start the demo first (`make demo`, then dist/demo/karots-suite-demo-room).
// DOCS_CODE = the hourly code (unlocks the demo and is the system user's PIN).
// DOCS_ONLY=<substring> retakes only the matching shots.
// Output: docs/images/<area>/<name>.jpg — stable names, the guides link to them.
const code = process.env.DOCS_CODE ?? ''
const only = process.env.DOCS_ONLY ?? ''
const out = '../docs/images'
const owner = { phone: '0771000001', pin: '1111' }
const cashier = { phone: '0772000001', pin: '4444' } // a sim cashier: their till is open
const system = { phone: '0000000001', pin: code }
const desk = { width: 1440, height: 900 }
const phone = { width: 390, height: 844 }

test.skip(!code, 'set DOCS_CODE to the Demo Room hourly code')
test.describe.configure({ mode: 'serial' })

// Hide the demo's own chrome (watermark tab, paper strip, logins) in every shot.
const hideChrome = () => {
  const s = document.createElement('style')
  s.textContent = '.ksd, [class^="ksd-"] { display: none !important }'
  document.addEventListener('DOMContentLoaded', () => document.head.appendChild(s))
}

async function unlock(page: Page) {
  const r = await page.request.post('/__demo/unlock', { data: { code } })
  if (!r.ok()) throw new Error(`demo unlock: ${r.status()} ${await r.text()}`)
}

type Who = { phone: string; pin: string }
async function session(browser: Browser, who: Who | null, viewport = desk, mode: 'light' | 'dark' = 'light') {
  const ctx = await browser.newContext({ viewport, baseURL: test.info().project.use.baseURL })
  await ctx.addInitScript((m) => localStorage.setItem('ks-mode', m), mode)
  await ctx.addInitScript(hideChrome)
  const page = await ctx.newPage()
  await unlock(page)
  if (who) {
    let r = await page.request.post('/api/v1/auth/login', { data: who })
    for (let i = 0; r.status() === 429 && i < 10; i++) { // the login limiter (burst 10, +1 per 2s)
      await page.waitForTimeout(2100)
      r = await page.request.post('/api/v1/auth/login', { data: who })
    }
    if (!r.ok()) throw new Error(`login ${who.phone}: ${r.status()} ${await r.text()}`)
  }
  return page
}

async function shot(page: Page, path: string, name: string, opts: { full?: boolean; prep?: (p: Page) => Promise<void> } = {}) {
  if (only && !name.includes(only)) return
  await page.goto(path)
  await page.waitForLoadState('networkidle')
  await opts.prep?.(page)
  await page.waitForTimeout(700) // charts and Motion settle
  await page.screenshot({ path: `${out}/${name}.jpg`, type: 'jpeg', quality: 82, fullPage: !!opts.full })
}

// Fill an empty screen so the shot shows the screen at work, not its blank state.
const pickFirst = (list: string) => async (p: Page) => {
  await p.locator(list.startsWith('ul') ? list : `ul[aria-label="${list}"]`).first().locator('li').first().click()
  await p.waitForLoadState('networkidle')
}
const findProduct = (q: string, list = 'Products') => async (p: Page) => {
  await p.getByLabel('Find a product').fill(q)
  await p.waitForLoadState('networkidle')
  await pickFirst(list)(p)
}
const scan = (codes: string[], field: string) => async (p: Page) => {
  for (const c of codes) {
    await p.getByLabel(field).fill(c)
    await p.getByLabel(field).press('Enter')
    await p.waitForLoadState('networkidle')
    await p.waitForTimeout(400)
  }
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
  if (st.phase !== 'ready' || st.shop?.key !== 'everything') {
    await page.request.post('/__demo/start', { data: { shop: 'everything' } })
    await test.expect
      .poll(async () => (await (await page.request.get('/__demo/state')).json()).phase, { timeout: 240_000, intervals: [2000] })
      .toBe('ready')
  }
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
