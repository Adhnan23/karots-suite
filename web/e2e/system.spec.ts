import { expect, test } from './fixtures'

test('shop admin cannot reach system appearance', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/admin/)
  await page.goto('/admin/system/appearance')
  await expect(page.getByText(/not found/i)).toBeVisible()
})

test('system user changes brand colour and it applies live', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0000000001')
  await page.getByLabel('PIN').fill(process.env.POS_SYSTEM_PIN ?? '2273')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/admin/)
  await page.goto('/admin/system/appearance')
  await page.getByLabel(/brand colou?r/i).fill('#0f766e')
  await page.getByRole('button', { name: /save/i }).click()
  await expect(page.getByText(/saved/i)).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.style.getPropertyValue('--brand'))).toBe('#0f766e')
  await page.getByLabel(/brand colou?r/i).fill('')
  await page.getByRole('button', { name: /save/i }).click()
})
