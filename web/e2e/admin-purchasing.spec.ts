import { expect, test, type Page } from './fixtures'

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
  await toast(page, new RegExp(`Paid Rs\\. 400\\.00 to ${name}`))
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
  await toast(page, /Received Rs\. 100\.00 back/)

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
