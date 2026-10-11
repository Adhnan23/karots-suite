import { expect, freshProduct, test, type Page } from './fixtures'

// Clearance (plugin, built in for Playwright). Needs `make e2e-seed`. Serial: one drawer.
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const slow = `Clr Slow ${tag}`
const sleepy = `Clr Sleepy ${tag}`

async function login(page: Page, phone: string, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}

const row = (page: Page, name: string) => page.getByRole('table', { name: 'To mark down' }).getByRole('row').filter({ hasText: name })

test('admin finds a slow product from the Apps tile and approves a markdown', async ({ page }) => {
  await login(page, '0770000001', '/admin')
  for (const [name, qty] of [[slow, '2'], [sleepy, '4']] as const) {
    const id = await freshProduct(page, name) // Rs 60, cost 40
    await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: id, newQuantity: qty, note: 'e2e', batchId: 0 } })
  }
  await page.getByRole('button', { name: 'Apps' }).click()
  await page.getByRole('link', { name: /Clearance/ }).click()
  await expect(page).toHaveURL(/\/admin\/x\/clearance$/)

  const r = row(page, slow)
  await expect(r).toContainText('never sold')
  await r.getByLabel(`Markdown % for ${slow}`).fill('15')
  await expect(r).toContainText('Rs. 51.00') // 60 − 15%
  await r.getByRole('button', { name: 'Approve' }).click()
  await expect(r).toContainText('Live −15%')
})

test('snooze hides a product until brought back', async ({ page }) => {
  await login(page, '0770000001', '/admin/x/clearance')
  await row(page, sleepy).getByRole('button', { name: 'Snooze' }).click()
  await expect(row(page, sleepy)).toHaveCount(0)
  await page.getByRole('button', { name: 'Snoozed', exact: true }).click()
  await expect(row(page, sleepy)).toContainText('snoozed to')
  await row(page, sleepy).getByRole('button', { name: 'Bring back' }).click()
  await page.getByRole('button', { name: 'All', exact: true }).click()
  await expect(row(page, sleepy)).toContainText('never sold')
})

test('the till offers the markdown, and it ends once the stock is sold', async ({ page }) => {
  await login(page, '0770000002', '/till')
  const dlg = page.getByRole('dialog', { name: 'Open the till' })
  if (await dlg.isVisible()) {
    await dlg.getByLabel(/Count of .*500\.00/).fill('2')
    await dlg.getByRole('button', { name: 'Open the till' }).click()
    await expect(dlg).toBeHidden()
    const skip = page.getByRole('button', { name: 'Skip' })
    await skip.waitFor({ state: 'visible', timeout: 1500 }).then(() => skip.click()).catch(() => undefined)
  }
  const search = page.getByLabel('Scan a barcode or search products')
  await search.fill(slow)
  await expect(page.getByText('Clearance -15%').first()).toBeVisible() // the card badge
  await page.getByRole('button', { name: new RegExp(slow) }).first().click()
  await page.getByRole('button', { name: 'Apply' }).click()
  // Both units, so the stock it was approved for is gone.
  await search.fill(slow)
  await page.getByRole('button', { name: new RegExp(slow) }).first().click()
  const again = page.getByRole('button', { name: 'Apply' })
  if (await again.isVisible().catch(() => false)) await again.click()
  await expect(page.getByText('Rs. 102.00').first()).toBeVisible() // 2 × 51
  const amount = page.getByLabel('Amount tendered')
  await amount.fill('200')
  await amount.press('Enter')
  await amount.press('Enter')
  await expect(page.getByRole('dialog', { name: /complete/ })).toBeVisible()
  // Leave the drawer as the till specs expect to find it: closed.
  await page.request.post('/api/v1/till/drawer/close', { data: { breakdown: {}, destLockerId: 0 } })

  await login(page, '0770000001', '/admin/x/clearance')
  await expect(row(page, slow)).toHaveCount(0) // sold out: no longer live, nothing left to mark down
  const id = (await (await page.request.get(`/api/v1/till/products?search=${encodeURIComponent(slow)}`)).json()).items[0].id
  const p = await (await page.request.get(`/api/v1/till/products/${id}`)).json()
  expect(p.badges ?? []).toEqual([])
})

test('dead stock: disable all shown, behind a confirm', async ({ page }) => {
  await login(page, '0770000001', '/admin/x/clearance/dead')
  const list = page.getByRole('table', { name: 'Dead stock' })
  await expect(list).toContainText('E2E Old Torch')
  await expect(list).toContainText('E2E Old Gloves')
  await page.getByRole('button', { name: /Disable all shown/ }).click()
  await page.getByRole('dialog').getByRole('button', { name: /^Disable \d+$/ }).click()
  await expect(page.getByText('No dead stock')).toBeVisible()
  const found = await (await page.request.get('/api/v1/till/products?search=E2E%20Old')).json()
  expect(found.items.length).toBe(0)
})

test('the page fits a phone', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 })
  await login(page, '0770000001', '/admin/x/clearance')
  await expect(page.getByRole('heading', { name: 'Clearance' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
})
