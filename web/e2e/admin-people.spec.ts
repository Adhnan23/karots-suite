import { expect, test, type Page, slipAsked } from './fixtures'

// M3 people admin flows (customers, users). Needs `make e2e-seed`.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString(36).slice(-5)
const phoneTag = String(Date.now()).slice(-7)

async function admin(page: Page, path: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(path)
  await page.waitForLoadState('networkidle')
}
async function pick(page: Page, label: string, text: string) {
  const sel = page.getByLabel(label)
  await sel.selectOption((await sel.locator('option', { hasText: text }).first().getAttribute('value'))!)
}
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()

test('customers: add with old debt over the limit, collect cash into the safe, edit, old balance, statement, disable', async ({ page }) => {
  await admin(page, '/admin/customers')
  const name = `Kamala ${tag}`
  const phone = `071${phoneTag}`
  await page.getByRole('button', { name: 'New customer' }).click()
  await page.getByLabel('Name').fill(name)
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('Credit limit').fill('1000')
  await page.getByLabel('Already owes').fill('1500')
  await page.getByRole('button', { name: 'Add customer' }).click()
  await toast(page, new RegExp(`${name} added`))
  await page.waitForURL(/\/admin\/customers\/\d+/)
  await expect(page.getByText('over limit').first()).toBeVisible()

  await page.getByRole('button', { name: '↓ Collect' }).click()
  await page.getByLabel('Amount received').fill('700')
  await pick(page, 'Cash into', 'E2E Safe')
  await page.getByRole('button', { name: /↓ Collect Rs\. 700\.00/ }).click()
  await slipAsked(page, /Collected Rs\. 700\.00/)
  await expect(page.getByLabel('Latest activity').getByText('Payment')).toBeVisible()
  await expect(page.getByText('Rs. 200.00 left of a Rs. 1,000.00 limit')).toBeVisible()

  await page.getByRole('button', { name: 'Edit' }).click()
  await page.getByLabel('Address').fill('Kandy Road')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await toast(page, /Customer updated/)

  await page.getByRole('button', { name: 'More' }).click()
  await page.getByRole('menuitem', { name: 'Old balance' }).click()
  await page.getByLabel('Old balance', { exact: true }).fill('300')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await toast(page, /Old balance corrected/)

  const id = page.url().split('/').pop()
  await page.goto(`/print/statement/customer/${id}`)
  await expect(page.getByText('Customer statement')).toBeVisible()
  await page.goBack()

  await page.getByRole('button', { name: 'More' }).click()
  await page.getByRole('menuitem', { name: 'Disable' }).click()
  await page.getByRole('button', { name: 'Disable', exact: true }).click()
  await toast(page, /disabled/)
  await page.getByRole('button', { name: 'More' }).click()
  await page.getByRole('menuitem', { name: 'Re-enable' }).click()
  await toast(page, /re-enabled/)

  await page.goto(`/admin/customers?q=${phone}`)
  await expect(page.getByRole('button', { name: new RegExp(name) }).first()).toBeVisible()
  await page.request.post('/api/v1/admin/customers', { data: { name: 'Dup', phone, address: '', creditLimit: '', openingBalance: '' } }).then((r) => expect(r.status()).toBe(409))
})

test('users: add a cashier with counter rights, they sign in; reset their PIN; deactivate; managers never see Users', async ({ page, browser }) => {
  await admin(page, '/admin/users')
  const phone = `076${phoneTag}`
  await page.getByRole('button', { name: 'New user' }).click()
  await page.getByLabel('Name').fill(`Nimali ${tag}`)
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('4321')
  await page.locator('label', { hasText: 'Counter operations' }).click()
  await page.getByRole('button', { name: 'Add user' }).click()
  await toast(page, new RegExp(`Nimali ${tag} can now sign in`))
  const card = page.getByRole('button', { name: new RegExp(`Nimali ${tag}`) })
  await expect(card.getByText('Counter operations')).toBeVisible()

  const signIn = async (pin: string) => {
    const ctx = await browser.newContext()
    const r = await ctx.request.post('/api/v1/auth/login', { data: { phone, pin } })
    await ctx.close()
    return r.status()
  }
  expect(await signIn('4321')).toBe(200)
  await card.click()
  await page.getByLabel('PIN').fill('8765')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await toast(page, /new PIN set/)
  expect(await signIn('4321')).toBe(401)
  expect(await signIn('8765')).toBe(200)

  await card.click()
  await page.getByRole('button', { name: 'Deactivate' }).click()
  await page.getByRole('button', { name: 'Deactivate', exact: true }).last().click()
  await toast(page, /can no longer sign in/)
  expect(await signIn('8765')).not.toBe(200)

  // A manager: no Users in the nav, and the page sends them back.
  await page.context().clearCookies()
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000003')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto('/admin/users')
  await page.waitForURL(/\/admin\/?$/)
})
