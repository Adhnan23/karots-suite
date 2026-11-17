import type { Page } from '@playwright/test'
import { expect, test } from './fixtures'

async function signIn(page: Page, phone: string, pin = '1234') {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill(pin)
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}

// Sell page 2.0: a till's screen size shows up in Appearance with who used it; the
// system user picks an arrangement for it, and that till shows it; Reset puts it back.
test('set a Sell page layout for a screen size the shop has', async ({ page, browser }) => {
  test.setTimeout(120_000)
  const till = await browser.newPage({ viewport: { width: 1366, height: 768 } })
  await signIn(till, '0770000002')
  const st = await (await till.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) await till.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
  await till.goto('/till')
  await expect(till.getByRole('region', { name: 'Payment' })).toBeVisible()

  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page, '0000000001', process.env.POS_SYSTEM_PIN ?? '2273')
  try {
    await page.goto('/admin/system/appearance')
    const card = page.getByRole('region', { name: 'Sell page layout' })
    await card.getByText('Sell page layout').click()
    const size = card.getByRole('button', { name: /1366 × 768/ })
    await expect(size).toContainText(/E2E/) // who used it
    await size.click()
    await card.getByRole('button', { name: 'Cart full height', exact: true }).click()
    await card.getByRole('button', { name: 'Small', exact: true }).click()
    await card.getByRole('button', { name: 'Save layouts' }).click()
    await expect(page.getByText('Layouts saved')).toBeVisible()

    await page.reload() // the card remembers it was open on this device
    await page.getByRole('region', { name: 'Sell page layout' }).getByRole('button', { name: /1366 × 768/ }).click()
    await expect(page.getByRole('region', { name: 'Sell page layout' }).getByRole('button', { name: 'Cart full height', exact: true })).toHaveAttribute('aria-pressed', 'true')

    await till.reload()
    const box = async (name: string) => (await till.getByRole('region', { name }).boundingBox())!
    await expect(till.getByRole('region', { name: 'Payment' })).toBeVisible()
    expect((await box('Cart')).x).toBeGreaterThan((await box('Catalogue')).x)
    expect((await box('Payment')).y).toBeGreaterThan((await box('Catalogue')).y + 100)

    await page.getByRole('region', { name: 'Sell page layout' }).getByRole('button', { name: 'Reset this size' }).click()
    await page.getByRole('region', { name: 'Sell page layout' }).getByRole('button', { name: 'Save layouts' }).click()
    await till.reload()
    await expect(till.getByRole('region', { name: 'Payment' })).toBeVisible()
    expect((await box('Payment')).x).toBeGreaterThan((await box('Cart')).x) // today's: products | cart | pay
  } finally {
    await page.request.put('/api/v1/system/sell-layout', { data: { layouts: {} } })
    await till.close()
  }
})

// Search only: no products column. Typing drops a list under the search bar and Enter adds;
// F1 opens Browse (the groups) and closes it again; plugin menus sit on the button row.
test('Search only: the search bar drops a list, F1 opens Browse', async ({ page, browser }) => {
  test.setTimeout(120_000)
  const till = await browser.newPage({ viewport: { width: 1366, height: 768 } })
  await signIn(till, '0770000002')
  const st = await (await till.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) await till.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
  await signIn(page, '0000000001', process.env.POS_SYSTEM_PIN ?? '2273')
  try {
    const put = await page.request.put('/api/v1/system/sell-layout', { data: { layouts: { '1366x768': { arr: 'CY', fr: [0.58, 0.42], split: 0, card: 'L', text: 100 } } } })
    expect(put.ok()).toBe(true)
    await till.goto('/till')
    await expect(till.getByRole('region', { name: 'Payment' })).toBeVisible()
    await expect(till.getByRole('toolbar', { name: 'Groups' })).toHaveCount(0) // no products column
    const box = async (name: string) => (await till.getByRole('region', { name }).boundingBox())!
    expect((await box('Payment')).x).toBeGreaterThan((await box('Cart')).x)
    expect((await box('Cart')).y).toBeGreaterThan((await box('Catalogue')).y)

    const search = till.locator('#till-search')
    await search.fill('Shelf Fan')
    await expect(till.getByRole('region', { name: 'Catalogue' }).getByRole('button', { name: /^E2E Shelf Fan/ })).toBeVisible()
    await search.press('ArrowDown')
    await search.press('Enter')
    await expect(till.getByRole('region', { name: 'Cart' }).getByText('1 items')).toBeVisible()

    await till.keyboard.press('F1')
    const browse = till.getByRole('dialog', { name: 'Browse' })
    await expect(browse).toBeVisible()
    await expect(browse.getByRole('toolbar', { name: 'Groups' })).toBeVisible()
    await till.keyboard.press('F1')
    await expect(browse).toHaveCount(0)
    await expect(search).toBeFocused()
  } finally {
    await page.request.put('/api/v1/system/sell-layout', { data: { layouts: {} } })
    await till.close()
  }
})
