import { expect, test, type Page, slipAsked } from './fixtures'

// M4 admin Sales: filter, then a return whose cash refund leaves a picked
// locker. Needs `make e2e-seed`.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString(36).slice(-5)
const safe = `Refund Safe ${tag}`

async function admin(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()
const tile = (page: Page, name: string) => page.getByRole('listitem').filter({ has: page.getByRole('link', { name, exact: true }) })

test('sales: find a cash sale, return it with the refund paid from a locker', async ({ page }) => {
  await admin(page)
  await page.request.post('/api/v1/admin/lockers', { data: { name: safe, kind: 'safe', openingBalance: '5000', overdraft: false, cashierAccess: false } })
  if (!(await (await page.request.get('/api/v1/till/drawer')).json()).summary.session)
    await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
  const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
  const sale = await (await page.request.post('/api/v1/till/sales', { data: { saleType: 'retail', discountType: 'fixed',
    items: [{ productId: milk.id, quantity: '2', discountType: 'fixed' }], payments: [{ method: 'cash', amount: '5000' }] } })).json()
  const refund = (Number(sale.total) / 2).toLocaleString('en-US', { minimumFractionDigits: 2 })

  await page.goto('/admin/sales')
  await page.getByLabel('Payment').selectOption('cash')
  await page.getByLabel('Search sales').fill(sale.receiptNo)
  await page.getByLabel('Sales').locator('visible=true').getByText(sale.receiptNo).click()
  await page.getByRole('button', { name: 'Return goods' }).click()
  await expect(page.getByRole('button', { name: 'Choose what comes back' })).toBeDisabled()
  await page.getByRole('button', { name: 'More E2E Milk' }).click()
  await expect(page.getByText('↑ Cash back')).toBeVisible()
  const pay = page.getByRole('button', { name: `↑ Refund Rs. ${refund}` })
  await expect(pay).toBeDisabled() // no source picked yet
  const src = page.getByLabel('Refund cash from')
  await src.selectOption((await src.locator('option', { hasText: safe }).getAttribute('value'))!)
  await pay.click()
  await slipAsked(page, new RegExp(`Returned — Rs\\. ${refund} refunded`))

  await page.getByLabel('Status').selectOption('partially_returned')
  await expect(page.getByLabel('Sales').locator('visible=true').getByText(sale.receiptNo)).toBeVisible()

  const csv = await (await page.request.get(`/api/v1/admin/sales/export?format=csv&q=${sale.receiptNo}`)).text()
  expect(csv).toContain(sale.receiptNo)

  await page.goto('/admin/lockers')
  const left = (5000 - Number(sale.total) / 2).toLocaleString('en-US', { minimumFractionDigits: 2 })
  await expect(tile(page, safe)).toContainText(`Rs. ${left}`)
})

test.describe('at 360px', () => {
  test.use({ viewport: { width: 360, height: 780 } })
  test('the sales list reads as cards on a phone', async ({ page }) => {
    await admin(page)
    await page.goto('/admin/sales')
    await expect(page.getByRole('region', { name: 'Totals' })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360)
  })
})
