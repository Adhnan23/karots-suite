import { expect, test, type Page } from './fixtures'

// Repairs (plugin, built in for Playwright). Needs `make e2e-seed`. Serial: one
// drawer, and later flows use earlier data.
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const PART = `A12 screen ${tag}`
const NAME = `Rep Kamal ${tag}`
const PHONE = `07${tag}88`
const ids = { part: 0, customer: 0 }

const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()
const json = async <T,>(p: Promise<{ json: () => Promise<unknown> }>) => (await (await p).json()) as T

async function login(page: Page, phone: string, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}

const search = (page: Page) => page.getByLabel('Scan a barcode or search products')
const cart = (page: Page) => page.getByRole('region', { name: 'Cart' })
const panel = (page: Page) => page.getByRole('dialog').filter({ hasText: /Repairs|repair job|Quick fix/ })

async function till(page: Page) {
  await login(page, '0770000002', '/till')
  const dlg = page.getByRole('dialog', { name: 'Open the till' })
  if (await dlg.isVisible()) {
    await dlg.getByLabel(/Count of .*500\.00/).fill('2')
    await dlg.getByRole('button', { name: 'Open the till' }).click()
    await expect(dlg).toBeHidden()
    const skip = page.getByRole('button', { name: 'Skip' })
    await skip.waitFor({ state: 'visible', timeout: 1500 }).then(() => skip.click()).catch(() => undefined)
  }
  if (await cart(page).getByText(/Cart is empty/).isHidden()) {
    await page.getByRole('button', { name: 'New sale' }).click()
    const yes = page.getByRole('dialog').getByRole('button', { name: /new sale|clear/i })
    if (await yes.isVisible().catch(() => false)) await yes.click()
  }
}

/** Skip the "print?" prompt if the shop asks before printing. */
async function skipPrint(page: Page) {
  const skip = page.getByRole('button', { name: 'Skip' })
  await skip.waitFor({ state: 'visible', timeout: 1500 }).then(() => skip.click()).catch(() => undefined)
}

async function newJob(page: Page, devicePlace: string, warranty = '') {
  await page.getByRole('button', { name: '🔧 Repairs' }).click()
  await panel(page).getByRole('button', { name: 'New job' }).click()
  const d = panel(page)
  await d.getByLabel('Find customer').fill(NAME)
  await d.getByRole('list', { name: 'Customers' }).getByRole('button', { name: new RegExp(NAME) }).click()
  await d.getByPlaceholder('e.g. Galaxy A12').fill(devicePlace)
  await d.getByPlaceholder(/What's wrong/).fill('Cracked screen')
  if (warranty) await d.getByLabel('Warranty (days)').fill(warranty)
  await d.getByRole('button', { name: 'Take it in' }).click()
  await toast(page, /taken in/)
}

async function deposit(page: Page, amount: string) {
  const d = panel(page)
  await d.getByRole('button', { name: 'Take a deposit' }).click()
  await d.getByLabel('Amount', { exact: true }).fill(amount)
  await d.getByRole('button', { name: /^Collect/ }).click()
  await skipPrint(page)
  await expect(d.getByText(/Advance on account/)).toBeVisible()
}

async function complete(page: Page) {
  await page.getByRole('button', { name: /Complete sale/ }).click()
  await expect(page.getByRole('dialog', { name: /complete/ })).toBeVisible()
  await page.keyboard.press('Enter')
}

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage()
  await login(page, '0770000001', '/admin')
  const cats = await json<{ id: number; name: string }[]>(page.request.get('/api/v1/admin/categories'))
  const res = await page.request.post('/api/v1/admin/products', { data: {
    name: PART, nameLocal: '', barcode: '', categoryId: cats.find((c) => c.name === 'E2E Goods')!.id, unitId: 1, costPrice: '2500', sellingPrice: '4000',
    wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } })
  expect(res.ok(), await res.text()).toBe(true)
  ids.part = (await res.json()).id
  expect((await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: ids.part, newQuantity: '10', note: 'repairs e2e', batchId: 0 } })).ok()).toBe(true)
  const c = await json<{ customer: { id: number } }>(page.request.post('/api/v1/till/customers', { data: { name: NAME, phone: PHONE, creditLimit: '0' } }))
  ids.customer = c.customer.id
  await page.close()
})

test('a job with a deposit is picked up on the normal Pay: the deposit pre-filled on account, the job collected', async ({ page }) => {
  await till(page)
  await newJob(page, `Galaxy ${tag}`, '30')
  const d = panel(page)
  await d.getByLabel('Add a part').fill(PART)
  await d.getByRole('button', { name: new RegExp(PART) }).click()
  await d.getByRole('button', { name: 'Add part' }).click()
  await d.getByLabel('Labour', { exact: true }).fill('Fitting')
  await d.getByLabel('Labour amount').fill('1500')
  await d.getByRole('button', { name: 'Add labour' }).click()
  await expect(d.getByText(/Estimate\s*Rs\. 5,500\.00/)).toBeVisible()
  await deposit(page, '1000')
  const ticket = (await d.locator('.font-display.text-lg').first().innerText()).trim()
  await page.keyboard.press('Escape')

  // Scanning the ticket on Sell opens its job.
  const job = await json<{ ticketCode: string }[]>(page.request.get(`/api/v1/x/repairs/jobs?show=all&q=${ticket}`))
  await search(page).fill(job[0].ticketCode)
  await search(page).press('Enter')
  await expect(panel(page).getByText(`Galaxy ${tag}`)).toBeVisible()
  await panel(page).getByRole('button', { name: 'Put on the bill' }).click()
  await toast(page, /is on the bill/)
  await expect(cart(page).getByText(PART)).toBeVisible()
  await expect(cart(page).getByText(`Fitting — ${ticket}`)).toBeVisible()
  await expect(cart(page).getByText(`🔧 ${ticket}`).first()).toBeVisible()
  // The deposit is already a payment row: 1,000 on account, 4,500 still to pay.
  await expect(page.getByText(/On account\s*Rs\. 1,000\.00/)).toBeVisible()
  await page.getByRole('button', { name: 'Cash', exact: true }).click()
  await page.getByRole('button', { name: 'Fill' }).click()
  await complete(page)
  await toast(page, new RegExp(`${ticket} collected`))
  const after = await json<{ status: string; outstanding: string; warrantyUntil: string; saleReceipt: string }[]>(page.request.get(`/api/v1/x/repairs/jobs?show=all&q=${ticket}`))
  expect(after[0].status).toBe('collected')
  const full = await json<{ outstanding: string; warrantyUntil: string; saleReceipt: string }>(page.request.get(`/api/v1/x/repairs/jobs/${(after[0] as unknown as { id: number }).id}`))
  expect(full.outstanding).toBe('0.00') // the advance was used
  expect(full.warrantyUntil).not.toBe('')
  expect(full.saleReceipt).toMatch(/^S-/)
})

test('a collected job in warranty is redone free: its lines go on the bill at 100% off', async ({ page }) => {
  await till(page)
  await page.getByRole('button', { name: '🔧 Repairs' }).click()
  await panel(page).getByRole('button', { name: 'Collected' }).click()
  await panel(page).getByRole('button', { name: new RegExp(`Galaxy ${tag}`) }).click()
  await panel(page).getByRole('button', { name: 'Free re-repair' }).click()
  await toast(page, /free under/)
  await panel(page).getByLabel('Add a part').fill(PART)
  await panel(page).getByRole('button', { name: new RegExp(PART) }).click()
  await panel(page).getByRole('button', { name: 'Add part' }).click()
  await panel(page).getByRole('button', { name: 'Put on the bill' }).click()
  await toast(page, /is on the bill/)
  await expect(page.getByText(/^Total$/).locator('..').getByText('Rs. 0.00')).toBeVisible()
  await complete(page)
  await toast(page, /collected/)
})

test('the sale itself collects the job — even when the cashier changes the customer at Pay', async ({ page }) => {
  await till(page)
  await newJob(page, `Redmi ${tag}`)
  const d = panel(page)
  await d.getByLabel('Labour', { exact: true }).fill('Port clean')
  await d.getByLabel('Labour amount').fill('300')
  await d.getByRole('button', { name: 'Add labour' }).click()
  await expect(d.getByText('Port clean')).toBeVisible()
  await d.getByRole('button', { name: 'Put on the bill' }).click()
  await toast(page, /is on the bill/)
  // Someone else pays: the cashier switches the sale to walk-in.
  await page.getByRole('button', { name: new RegExp(NAME) }).first().click()
  await page.getByRole('listbox', { name: 'Customers' }).getByRole('option', { name: 'Walk-in' }).click()
  await page.getByRole('button', { name: 'Cash', exact: true }).click()
  await page.getByRole('button', { name: 'Fill' }).click()
  await complete(page)
  await toast(page, /collected/)
  const rows = await json<{ id: number; status: string }[]>(page.request.get(`/api/v1/x/repairs/jobs?show=all&q=${encodeURIComponent(`Redmi ${tag}`)}`))
  expect(rows[0].status).toBe('collected')
})

test('a quick fix is a labour line on the bill — no job', async ({ page }) => {
  await till(page)
  await page.getByRole('button', { name: '🔧 Repairs' }).click()
  await panel(page).getByRole('button', { name: 'Quick fix' }).click()
  await panel(page).getByRole('button', { name: 'SIM reseat' }).click()
  await panel(page).getByRole('textbox', { name: 'Price', exact: true }).fill('200')
  await panel(page).getByRole('button', { name: 'Add to the bill' }).click()
  await expect(cart(page).getByText('Repair — SIM reseat')).toBeVisible()
  await page.getByRole('button', { name: 'New sale' }).click()
  const yes = page.getByRole('dialog').getByRole('button', { name: /new sale|clear/i })
  if (await yes.isVisible().catch(() => false)) await yes.click()
})

test('cancelling a job with a deposit offers it back through the core refund', async ({ page }) => {
  await till(page)
  await newJob(page, `iPhone ${tag}`)
  await deposit(page, '500')
  const d = panel(page)
  await d.getByRole('button', { name: 'Cancel job' }).click()
  await page.getByRole('button', { name: 'Cancel the job' }).click()
  await d.getByRole('button', { name: /Give back the deposit Rs\. 500\.00/ }).click()
  await d.getByRole('button', { name: /^Refund Rs\. 500\.00/ }).click()
  await skipPrint(page)
  await expect(d.getByRole('button', { name: /Give back the deposit/ })).toBeHidden()
  const c = await json<{ id: number; outstanding: string }[]>(page.request.get('/api/v1/till/customers'))
  expect(c.find((x) => x.id === ids.customer)!.outstanding).toBe('0.00')
})
