import { test, type Page } from './fixtures'

// UI-gate screenshots for the counter tabs (not a behaviour test). Needs `make e2e-seed`.
//   SHOTS=m1b-2 SHOTS_TAB=expenses bunx playwright test e2e/counter-shots.spec.ts
// SHOTS_PREP (optional) names a prep step below that fills the tab before the shot.
// Output: web/shots/<gate>/<tab>-<palette>-<mode>-<width>.png
const gate = process.env.SHOTS
const tab = process.env.SHOTS_TAB ?? 'expenses'
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
  // Reload & Bills: a dealer selling a carrier's float. SHOTS_TAB=suppliers SHOTS_PREP=float
  float: async (page) => {
    const post = async (url: string, data: object, method = 'post') => (await page.request.fetch(`/api/v1${url}`, { method, data })).json().catch(() => ({}))
    const sup = await post('/till/suppliers', { name: 'Float Dealer', phone: '0771234567' })
    const c = await post('/x/recharge/carriers', { name: 'Dialog' })
    await post(`/x/recharge/carriers/${c.id}/supplier`, { supplierId: sup.id }, 'put')
    await post('/x/recharge/devices', { carrierId: c.id, label: 'Shop phone', number: '', reload: true, money: true, target: '5000', opening: '1200' })
    const m = await post('/x/recharge/carriers', { name: 'Mobitel' })
    await post('/x/recharge/devices', { carrierId: m.id, label: 'Shop SIM', number: '', reload: true, money: false, target: '0', opening: '300' })
    await page.reload()
    await page.getByLabel('Find a supplier').fill('Float Dealer')
    await page.getByRole('list', { name: 'Suppliers' }).getByRole('button', { name: /Float Dealer/ }).first().click()
    await page.getByRole('group', { name: 'Job' }).getByRole('button', { name: '📲 Float' }).click()
    await page.getByLabel('Float bought').fill('2500')
    await page.getByLabel('Bill number').fill('F-2231')
  },
  expenses: async (page) => {
    await page.getByRole('button', { name: 'Transport', exact: true }).click()
    await page.getByLabel('Amount').fill('350')
    await page.getByLabel('Note').fill('Delivery tuk')
    await page.getByRole('button', { name: /Record expense/ }).click()
    await page.getByText('Delivery tuk').waitFor()
    await page.getByRole('button', { name: 'Electricity', exact: true }).click()
    await page.getByLabel('Amount').fill('1200')
  },
  'write-off': async (page) => {
    await page.getByLabel('Find a product').fill('Lot Soap')
    await page.getByRole('option').first().click()
    await page.getByRole('radio', { name: /Shop use/ }).click()
    await page.getByRole('radio', { name: /120\.00/ }).click()
    await page.getByLabel('Quantity').fill('2')
    await page.getByLabel('Note').fill('Washing the counter')
  },
  labels: async (page) => {
    await page.getByLabel('Find a product').fill('Milk')
    await page.getByRole('option', { name: /E2E Milk\b(?! Crate)/ }).click()
    await page.getByRole('button', { name: /40 × 30/ }).click()
    await page.getByLabel('Label preview').waitFor()
  },
  returns: async (page) => {
    const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
    const yog = await (await page.request.get('/api/v1/till/products/barcode/4790000000035')).json()
    const sale = await (await page.request.post('/api/v1/till/sales', { data: {
      saleType: 'retail', discountType: 'fixed', payments: [{ method: 'cash', amount: '500' }],
      items: [{ productId: milk.id, quantity: '3', discountType: 'fixed' }, { productId: yog.id, quantity: '1', discountType: 'fixed', allowOversell: true }],
    } })).json()
    await page.getByLabel('Find a sale').fill(sale.receiptNo)
    await page.getByLabel('Find a sale').press('Enter')
    await page.getByRole('button', { name: 'More E2E Milk' }).click()
    await page.getByRole('button', { name: 'More E2E Milk' }).click()
    await page.getByText(/Faulty — don't/).first().click()
    await page.getByRole('button', { name: 'Wrong item' }).click()
  },
  credit: async (page) => {
    const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
    const cust = (await (await page.request.get('/api/v1/till/customers')).json()).find((c: { name: string }) => c.name === 'E2E Credit')
    await page.request.post('/api/v1/till/sales', { data: {
      saleType: 'retail', discountType: 'fixed', customerId: cust.id, payments: [{ method: 'credit', amount: '300' }],
      items: [{ productId: milk.id, quantity: '3', discountType: 'fixed' }],
    } })
    await page.reload()
    await page.getByRole('button', { name: /E2E Credit/ }).click()
    await page.getByLabel('Amount').fill('200')
  },
  receipts: async (page) => {
    const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
    await page.request.post('/api/v1/till/sales', { data: {
      saleType: 'retail', discountType: 'fixed', payments: [{ method: 'cash', amount: '500' }],
      items: [{ productId: milk.id, quantity: '2', discountType: 'fixed' }],
    } })
    await page.reload()
    await page.getByLabel('Period').selectOption('this-week')
    await page.getByLabel('Find a sale').press('Enter')
    await page.getByText('TOTAL', { exact: true }).waitFor()
  },
  // SHOTS_TAB=returns SHOTS_PREP=older: a list past one page ends in "Show older".
  older: async (page) => {
    if (!(await (await page.request.get('/api/v1/till/sales?q=&offset=50')).json()).length) {
      const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
      for (let i = 0; i < 51; i++) {
        await page.request.post('/api/v1/till/sales', { data: {
          saleType: 'retail', discountType: 'fixed', payments: [{ method: 'cash', amount: '200' }],
          items: [{ productId: milk.id, quantity: '1', discountType: 'fixed' }],
        } })
      }
      await page.reload()
    }
    const list = page.getByRole('list', { name: 'Sales' })
    await page.getByRole('button', { name: 'Show older' }).waitFor()
    await list.evaluate((el) => { el.scrollTop = el.scrollHeight })
  },
  zreport: async (page) => {
    const st = await (await page.request.get('/api/v1/till/drawer')).json()
    await page.goto(`/print/z/${st.summary.session.id}`)
    await page.getByText('Cash movements').waitFor()
  },
  warranty: async (page) => {
    await page.getByLabel('Serial number').fill('E2E-SN-0001')
    await page.getByLabel('Serial number').press('Enter')
    await page.getByLabel('New serial').fill('E2E-SN-0002')
    await page.getByRole('button', { name: 'Not charging' }).click()
  },
  suppliers: async (page) => {
    const sup = (await (await page.request.get('/api/v1/till/suppliers?q=E2E%20Distributors')).json())[0]
    const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
    const soap = await (await page.request.get('/api/v1/till/products/barcode/4790000000028')).json()
    await page.request.post(`/api/v1/till/suppliers/${sup.id}/order`, { data: { notes: '', lines: [
      { productId: milk.id, quantity: '24' }, { productId: soap.id, quantity: '12' }] } })
    await page.getByRole('button', { name: /E2E Distributors/ }).click()
    await page.getByRole('button', { name: 'Receive stock' }).click()
    await page.getByRole('button', { name: /^#\d+/ }).first().click()
    await page.getByLabel('Quantity of E2E Lot Soap').fill('10')
    await page.getByLabel('Discount on E2E Milk', { exact: true }).fill('5')
    await page.getByLabel('Bill discount', { exact: true }).fill('100')
    await page.getByLabel('Pay now').fill('1000')
  },
  convert: async (page) => {
    await page.getByRole('button', { name: /Milk Crate/ }).click()
    await page.getByLabel('Packs to open').fill('2')
  },
}

for (const palette of (process.env.SHOTS_PALETTES ?? 'studio,night').split(',')) {
  for (const mode of ['light', 'dark']) {
    for (const width of [1366, 1024]) {
      test(`${palette} ${mode} ${width}`, async ({ page }) => {
        await page.setViewportSize({ width, height: 768 })
        await login(page, '0000000001', process.env.POS_SYSTEM_PIN ?? '2273')
        const cur = await (await page.request.get('/api/v1/system/appearance')).json()
        await page.request.put('/api/v1/system/appearance', { data: { ...cur, palette } })
        await page.context().clearCookies()
        await page.addInitScript((m) => localStorage.setItem('ks-mode', m), mode)
        await login(page, '0770000001', '1234') // admin: sees every tab
        const st = await (await page.request.get('/api/v1/till/drawer')).json()
        if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 4 }, sourceLockerId: 0 } })
        await page.goto(`/till/${tab}`)
        await page.waitForLoadState('networkidle')
        await prep[process.env.SHOTS_PREP ?? tab]?.(page)
        await page.waitForTimeout(300)
        await page.screenshot({ path: `shots/${gate}/${tab}-${palette}-${mode}-${width}.png` })
      })
    }
  }
}
