import { expect, freshProduct, test, type Page } from './fixtures'

// How early a lot warns: the shop's number in Settings, or its supplier's own.
// Needs `make e2e-seed`.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString(36).slice(-5)
const name = `E2E Expiry ${tag}`

async function signIn(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000003')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}
const settings = async (page: Page) => (await (await page.request.get('/api/v1/admin/settings')).json()).settings
const row = (page: Page) => page.getByRole('list', { name: 'Products with lots' }).getByRole('listitem').filter({ hasText: name }).first()

test('a lot warns by its supplier, and a blank supplier follows Settings', async ({ page }) => {
  await signIn(page)
  const before = await settings(page)
  try {
    const sup = await (await page.request.post('/api/v1/admin/suppliers', { data: { name: `E2E Notice ${tag}`, contact: '', phone: '', address: '', creditDays: 0, openingBalance: '' } })).json()
    const pid = await freshProduct(page, name)
    const expiry = new Date(Date.now() + 40 * 86_400_000).toISOString().slice(0, 10)
    const res = await page.request.post(`/api/v1/till/suppliers/${sup.id}/receive`, { data: {
      invoiceNo: '', discount: '', discountType: 'fixed', notes: '', keepRemainder: false, deferDrawerKick: true,
      pay: { amount: '', method: 'cash', lockerId: 0 },
      lines: [{ productId: pid, name: '', ordered: '', quantity: '2', freeQty: '', costPrice: '40', sellingPrice: '', expiryDate: expiry, discount: '', discountType: 'fixed' }],
    } })
    expect(res.ok()).toBeTruthy()

    // The shop's number is 10, so a lot 40 days out doesn't warn yet.
    await page.goto('/admin/settings')
    const stock = page.getByRole('region', { name: 'Stock' }).or(page.locator('section', { hasText: 'Stock-take & intake' })).first()
    await stock.getByLabel('Warn before expiry').fill('10')
    await stock.getByRole('button', { name: 'Save' }).click()
    await expect.poll(async () => (await settings(page)).expiryWarnDays).toBe(10)
    await page.goto('/admin/stock/batches')
    await expect(row(page)).toBeVisible()
    await expect(row(page)).not.toContainText('expiring')

    // The supplier asks for 60: now it warns, and says whose it is.
    await page.goto(`/admin/suppliers/${sup.id}`)
    await page.getByRole('button', { name: /^edit/i }).click()
    await page.getByRole('dialog').getByLabel('Warn before expiry').fill('60')
    await page.getByRole('button', { name: 'Save' }).click()
    await expect(page.getByText('warns 60 days before expiry')).toBeVisible()
    await page.goto('/admin/stock/batches')
    await expect(row(page)).toContainText('1 expiring')
    await expect(row(page)).toContainText(`E2E Notice ${tag}, warns 60 days before`)
  } finally {
    await page.request.put('/api/v1/admin/settings', { data: before })
  }
})
