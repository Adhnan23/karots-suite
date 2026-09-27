import { expect, test, type Page } from '@playwright/test'

// Seeded by `make e2e-seed` (Step 2): admin 0770000001 / cashier 0770000002, PIN 1234.
const login = async (page: Page, phone: string) => {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
}

test('bad cookie on load lands on login, not a blank page or loop', async ({ page, context }) => {
  await context.addCookies([{ name: 'pos_token', value: 'garbage', url: 'http://localhost:3100' }])
  await page.goto('/admin')
  await expect(page).toHaveURL(/\/login$/)
  await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible()
})

test('wrong PIN shows an inline error and stays on login', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('9999')
  await page.getByRole('button', { name: /sign in/i }).click()
  await expect(page.getByRole('alert')).toBeVisible()
  await expect(page).toHaveURL(/\/login$/)
})

test('konami on login reveals the install id', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByText(/install id/i)).toBeHidden()
  for (const k of ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a'])
    await page.keyboard.press(k)
  await expect(page.getByText(/^[0-9A-F]{8}$/)).toBeVisible()
})

test('admin lands on /admin; lock + reload stays locked; unlock resumes', async ({ page }) => {
  await login(page, '0770000001')
  await expect(page).toHaveURL(/\/admin/)
  await page.getByRole('button', { name: /lock/i }).click()
  await expect(page).toHaveURL(/\/lock$/)
  await page.goto('/admin')
  await expect(page).toHaveURL(/\/lock$/)
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /unlock/i }).click()
  await expect(page).toHaveURL(/\/admin/)
})

test('cashier lands on /till', async ({ page }) => {
  await login(page, '0770000002')
  await expect(page).toHaveURL(/\/till/)
})
