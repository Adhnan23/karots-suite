import { expect, test, type Page } from './fixtures'

async function signIn(page: Page, phone: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(admin|till)/)
}

test.setTimeout(90_000)

test("admin sees who's on live: the cashier at the till, locks it from the card, sees the game", async ({ page, browser }) => {
  const other = await browser.newContext() // its own localStorage = its own device
  const cashier = await other.newPage()
  await signIn(cashier, '0770000002')
  await cashier.goto('/till/returns')

  await signIn(page, '0770000001')
  const card = page.getByRole('region', { name: "Who's on now" })
  const row = card.getByRole('listitem').filter({ hasText: 'E2E Cashier' })
  await expect(row).toContainText('Till · Returns')
  await expect(row).toContainText('Active')

  // Lock it from the card: the cashier's screen locks on its next heartbeat (≤ 20 s).
  await row.getByRole('button', { name: 'Lock' }).click()
  await expect(cashier).toHaveURL(/\/lock$/, { timeout: 30_000 })
  await expect(row).toContainText('Locked', { timeout: 10_000 })
  await expect(row.getByRole('button')).toHaveCount(0)

  // A game on the lock screen shows up too.
  const clock = cashier.getByText(/^\d{1,2}:\d{2}/).first()
  for (let i = 0; i < 7; i++) await clock.click()
  await cashier.getByRole('dialog', { name: 'Games' }).getByRole('button', { name: /Blocks/ }).click()
  await expect(row).toContainText('Playing Blocks', { timeout: 10_000 })

  await page.goto('/admin/users')
  await expect(page.getByRole('listitem').filter({ hasText: 'E2E Cashier' })).toContainText('Locked')
  await other.close()
})
