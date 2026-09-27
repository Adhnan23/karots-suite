import { expect, test, type Page } from '@playwright/test'

async function loginAdmin(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await expect(page).toHaveURL(/\/admin/)
}

test('admin shell is usable at 360px (no horizontal scroll, bottom tabs)', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 })
  await loginAdmin(page)
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360)
  await expect(page.getByRole('navigation', { name: /primary/i })).toBeVisible()
})

test('admin shell shows sidebar at 1366px', async ({ page }) => {
  await page.setViewportSize({ width: 1366, height: 768 })
  await loginAdmin(page)
  await expect(page.getByRole('navigation', { name: /sidebar/i })).toBeVisible()
})

test('deep link reload survives', async ({ page }) => {
  await loginAdmin(page)
  await page.goto('/admin?x=1')
  await expect(page).toHaveURL(/\/admin/)
  await page.reload()
  await expect(page.getByText(/welcome/i)).toBeVisible()
})
