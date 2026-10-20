import { expect, test, type Page } from './fixtures'

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

test('seven taps on the clock open the games menu; a game opens, Esc comes back, Esc closes', async ({ page }) => {
  await page.goto('/login')
  const clock = page.getByText(/^\d{1,2}:\d{2}/).first()
  for (let i = 0; i < 7; i++) await clock.click()
  const menu = page.getByRole('dialog', { name: 'Games' })
  await expect(menu).toBeVisible()
  await menu.getByRole('button', { name: /Shop Rush/ }).click()
  await expect(page.getByRole('dialog', { name: 'Shop game' })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(menu).toBeVisible()
  await page.keyboard.press('2')
  await expect(page.getByRole('dialog', { name: 'Blocks game' })).toBeVisible()
  await page.keyboard.press('Escape')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.getByLabel('Phone')).toBeVisible()
})

test('admin lands on /admin; lock + reload stays locked; unlock resumes', async ({ page }) => {
  await login(page, '0770000001')
  await expect(page).toHaveURL(/\/admin/)
  await page.getByRole('button', { name: 'Lock', exact: true }).click() // the header's, not a Who's on now row's
  await expect(page).toHaveURL(/\/lock$/)
  await page.goto('/admin')
  await expect(page).toHaveURL(/\/lock$/)
  await expect(page.getByText(/Locked (just now|\d+ min ago)/)).toBeVisible()
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /unlock/i }).click()
  await expect(page).toHaveURL(/\/admin/)
})

test('cashier lands on /till', async ({ page }) => {
  await login(page, '0770000002')
  await expect(page).toHaveURL(/\/till/)
})

test('no browser memory: autocomplete off, no password field', async ({ page }) => {
  await page.goto('/login')
  await expect(page.locator('form')).toHaveAttribute('autocomplete', 'off')
  expect(await page.locator('input[type="password"]').count()).toBe(0)
})

test('the keypad writes into the PIN field and signs in', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').click()
  for (const d of ['1', '2', '3', '4']) await page.getByRole('button', { name: d, exact: true }).click()
  await page.getByRole('button', { name: /sign in/i }).click()
  await expect(page).toHaveURL(/\/admin/)
})
