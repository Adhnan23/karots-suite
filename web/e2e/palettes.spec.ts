import { expect, test, type Page } from './fixtures'

// Custom palettes (docs/specs/2026-10-21-palettes-design.md). Every test puts Studio back.
const SYSTEM_PIN = process.env.POS_SYSTEM_PIN ?? '2273'

async function systemLogin(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0000000001')
  await page.getByLabel('PIN').fill(SYSTEM_PIN)
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/admin/)
}

const BLUE = '#123456'
const css = (id: number) =>
  `/* karots-palette theme=dock name=E2E Blue base=studio */\n` +
  `:root[data-ui="dock"][data-palette="custom-${id}"] { --background: ${BLUE}; }\n` +
  `:root[data-ui="dock"][data-palette="custom-${id}"][data-theme="dark"] { --background: ${BLUE}; }\n`

test('the active custom palette shows on the login page, before and after sign-in', async ({ page, browser }) => {
  await systemLogin(page)
  const made = await (await page.request.post('/api/v1/system/palettes', { data: { name: 'E2E Blue', base: 'studio', css: '' } })).json()
  await page.request.put(`/api/v1/system/palettes/${made.id}`, { data: { name: 'E2E Blue', base: 'studio', css: css(made.id) } })
  await page.request.put('/api/v1/system/appearance', { data: { palette: `custom-${made.id}`, receiptStyle: 'classic' } })
  try {
    // This device signed in before: the custom palette paints with the first frame.
    await page.goto('/admin')
    await expect(page.locator('html')).toHaveAttribute('data-palette', `custom-${made.id}`)
    await page.request.post('/api/v1/auth/logout')
    await page.goto('/login', { waitUntil: 'domcontentloaded' })
    expect(await page.evaluate(() => getComputedStyle(document.body).backgroundColor)).toBe('rgb(18, 52, 86)')

    // A device that never signed in learns it from login-info.
    const fresh = await browser.newPage()
    await fresh.goto('/login')
    await expect.poll(() => fresh.evaluate(() => getComputedStyle(document.body).backgroundColor)).toBe('rgb(18, 52, 86)')
    await fresh.close()
  } finally {
    await systemLogin(page)
    await page.request.put('/api/v1/system/appearance', { data: { palette: 'studio', receiptStyle: 'classic' } })
    await page.request.delete(`/api/v1/system/palettes/${made.id}`)
  }
})
