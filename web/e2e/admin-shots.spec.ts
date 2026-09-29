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

const prep: Record<string, (page: Page) => Promise<void>> = {
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
    await page.getByLabel('Lots').waitFor()
  },
  adjust: async (page) => {
    await prep.lots(page)
    await page.getByRole('button', { name: 'Adjust the count' }).click()
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
