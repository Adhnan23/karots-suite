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
