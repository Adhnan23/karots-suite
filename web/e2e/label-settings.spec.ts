import { expect, test, type Page } from './fixtures'

// Settings → Printers & labels (spec 2026-10-26): "Print labels as" Image / Classic
// with a live label preview of unsaved choices; the A4 sheet follows the switch.
test.describe.configure({ mode: 'serial' })

async function asManager(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000003')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}
async function setLabelsAs(page: Page, labelPrintAs: string) {
  const s = (await (await page.request.get('/api/v1/admin/settings')).json()).settings
  expect((await page.request.put('/api/v1/admin/settings', { data: { ...s, labelPrintAs } })).ok()).toBe(true)
}

test.afterAll(async ({ browser }) => {
  const page = await browser.newPage()
  await asManager(page)
  await setLabelsAs(page, 'classic')
  await page.close()
})

test('the switch and the live label preview', async ({ page }) => {
  await asManager(page)
  await setLabelsAs(page, 'classic')
  await page.goto('/admin/settings')
  const sec = page.getByRole('region', { name: 'Printers & labels' })
  const classicSvg = sec.getByRole('img', { name: 'Label preview' })
  const picture = sec.getByRole('img', { name: 'Label as it prints' })

  // Classic: the printer geometry; a bigger second text wraps to more lines live, before Save.
  await expect(classicSvg).toBeVisible()
  await sec.getByRole('radiogroup', { name: 'Second text', exact: true }).getByRole('radio', { name: 'Name' }).click()
  await sec.getByRole('radiogroup', { name: 'Second text size' }).getByRole('radio', { name: 'L' }).click()
  await sec.getByRole('radiogroup', { name: 'Second text lines' }).getByRole('radio', { name: '1' }).click()
  await expect.poll(() => classicSvg.locator('text').count()).toBeGreaterThan(0)
  const before = await classicSvg.locator('text').count()
  await sec.getByRole('radiogroup', { name: 'Second text lines' }).getByRole('radio', { name: '3' }).click()
  await expect.poll(() => classicSvg.locator('text').count()).toBeGreaterThan(before)

  // Image: the picture, redrawn when the label size changes (before Save).
  await sec.getByRole('radiogroup', { name: 'Print labels as' }).getByRole('radio', { name: /Image/ }).click()
  await expect(picture).toHaveAttribute('src', /^blob:/)
  await expect(sec.getByText(/50 × 25 mm · as it prints/)).toBeVisible()
  await sec.getByLabel('Width mm').fill('40')
  await sec.getByLabel('Height mm').fill('30')
  await expect(sec.getByText(/40 × 30 mm · as it prints/)).toBeVisible()
  await expect(picture).toHaveAttribute('data-label-style', /.+/)
})

test('the A4 sheet draws the style in Image mode, the printer layout in Classic', async ({ page }) => {
  await asManager(page)
  const url = '/print/labels?code=4792024012348&format=EAN13&top=Milk&bottom=Rs%20100&topKey=name&bottomKey=price&count=3&size=50x25'
  await setLabelsAs(page, 'image')
  await page.goto(url)
  await expect(page.locator('[data-label-style]')).toHaveCount(3)
  await setLabelsAs(page, 'classic')
  await page.goto(url)
  await expect(page.getByRole('img', { name: 'Label 3' })).toBeVisible()
  await expect(page.locator('[data-label-style]')).toHaveCount(0)
})
