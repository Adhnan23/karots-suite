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
    await card.getByRole('button', { name: 'Cart full height' }).click()
    await card.getByRole('button', { name: 'Small', exact: true }).click()
    await card.getByRole('button', { name: 'Save layouts' }).click()
    await expect(page.getByText('Layouts saved')).toBeVisible()

    await page.reload() // the card remembers it was open on this device
    await page.getByRole('region', { name: 'Sell page layout' }).getByRole('button', { name: /1366 × 768/ }).click()
    await expect(page.getByRole('region', { name: 'Sell page layout' }).getByRole('button', { name: 'Cart full height' })).toHaveAttribute('aria-pressed', 'true')

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
