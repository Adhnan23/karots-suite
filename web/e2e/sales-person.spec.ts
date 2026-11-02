import { expect, test, type Page } from './fixtures'

// A sales person sends a cart to the cashier from another machine; the cashier gets a
// popup, opens the ticket, takes the money. Needs `make e2e-seed`.
async function login(page: Page, phone: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto('/till')
  await page.waitForLoadState('networkidle')
}

test('a sales person sends a ticket and the cashier sells it', async ({ browser }) => {
  const seller = await (await browser.newContext({ viewport: { width: 1366, height: 768 } })).newPage()
  const cashier = await (await browser.newContext({ viewport: { width: 1366, height: 768 } })).newPage()
  await login(cashier, '0770000003') // the manager's till stands in for the cashier's
  const st = await (await cashier.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) {
    await cashier.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
    await cashier.reload()
  }
  await login(seller, '0770000004')

  // Same sell screen, no money anywhere.
  await expect(seller.getByRole('dialog', { name: 'Open the till' })).toHaveCount(0)
  await expect(seller.getByRole('button', { name: /Complete sale/ })).toHaveCount(0)
  for (const name of ['Deposit', 'Withdraw', 'Close till']) await expect(seller.getByRole('button', { name })).toHaveCount(0)
  await expect(seller.getByLabel('Bill discount', { exact: true })).toHaveCount(0)
  expect((await seller.request.post('/api/v1/till/sales', { data: {} })).status()).toBe(403)
  expect((await seller.request.post('/api/v1/till/drawer/open', { data: {} })).status()).toBe(403)

  await seller.locator('#till-search').fill('4790000000011')
  await seller.keyboard.press('Enter')
  const cart = seller.getByRole('region', { name: 'Cart' })
  await expect(cart.getByText('E2E Milk')).toBeVisible()
  await seller.screenshot({ path: 'shots/sales-person/sell.png' })
  await seller.getByLabel('Name or note').fill('Blue shirt')
  const send = seller.getByRole('button', { name: 'Send to E2E Manager' })
  await send.click()
  const sent = seller.locator('[data-sonner-toast]').filter({ hasText: /Ticket \d+ sent to E2E Manager/ }).first()
  await expect(sent).toBeVisible()
  await expect(send).toBeDisabled() // the cart is free for the next customer
  await expect(seller.getByRole('group', { name: 'Tickets' })).toContainText('E2E Milk ×1')
  await expect(seller.getByLabel('Last ticket')).toContainText(/\d\d/)
  await seller.waitForTimeout(800)
  await seller.screenshot({ path: 'shots/sales-person/sent.png' })

  // Wrong cashier: Undo brings the cart back, and it goes again.
  await sent.getByRole('button', { name: 'Undo' }).click()
  await expect(send).toBeEnabled()
  await expect(seller.getByRole('group', { name: 'Tickets' })).toHaveCount(0)
  await seller.getByLabel('Name or note').fill('Blue shirt')
  await send.click()
  await expect(send).toBeDisabled()

  // The popup, without a reload; the ticket also waits under the cart.
  const popup = cashier.locator('[data-sonner-toast]').filter({ hasText: /Ticket \d+ · Blue shirt · from E2E Sales/ }).first()
  await expect(popup).toBeVisible({ timeout: 10_000 })
  await cashier.waitForTimeout(800) // the popup has finished sliding in
  await cashier.screenshot({ path: 'shots/sales-person/popup.png' })
  await expect(cashier.getByRole('group', { name: 'Tickets' })).toContainText('E2E Sales')
  await popup.getByRole('button', { name: 'Open' }).click()
  await expect(cashier.getByRole('region', { name: 'Cart' }).getByText('E2E Milk')).toBeVisible()

  // Opened once: it's gone from both machines.
  await expect(cashier.getByRole('group', { name: 'Tickets' })).toHaveCount(0)
  await expect(seller.getByRole('group', { name: 'Tickets' })).toHaveCount(0, { timeout: 10_000 })

  // The money goes the normal way, and the sale records both people.
  await cashier.locator('#till-amount').fill('100')
  await cashier.getByRole('button', { name: /Complete sale/ }).click()
  await expect.poll(async () => {
    const rows = await (await cashier.request.get('/api/v1/admin/sales')).json()
    return (rows.items ?? rows)[0]?.preparedBy
  }, { timeout: 10_000 }).toBe('E2E Sales')
})

test('a till closes only once its tickets are dealt with', async ({ browser }) => {
  const seller = await (await browser.newContext({ viewport: { width: 1366, height: 768 } })).newPage()
  const cashier = await (await browser.newContext({ viewport: { width: 1366, height: 768 } })).newPage()
  await login(cashier, '0770000003')
  const st = await (await cashier.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) {
    await cashier.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
    await cashier.reload()
  }
  await login(seller, '0770000004')
  await seller.locator('#till-search').fill('4790000000011')
  await seller.keyboard.press('Enter')
  await seller.getByRole('button', { name: 'Send to E2E Manager' }).click()
  await expect(seller.getByRole('group', { name: 'Tickets' })).toBeVisible()

  await expect(cashier.getByRole('group', { name: 'Tickets' })).toBeVisible({ timeout: 10_000 })
  await cashier.getByRole('button', { name: 'Close till' }).click()
  const dlg = cashier.getByRole('dialog', { name: 'Tickets waiting on your till' })
  await expect(dlg).toBeVisible()
  await cashier.waitForTimeout(400)
  await cashier.screenshot({ path: 'shots/sales-person/close.png' })
  await dlg.getByRole('button', { name: 'Back to E2E Sales' }).click()
  await expect(dlg).toHaveCount(0)
  await expect(cashier.getByRole('dialog')).toContainText(/count|drawer/i) // on to the count
  await cashier.keyboard.press('Escape')

  await seller.reload() // Held only refreshes on this machine's own actions
  await expect(seller.getByRole('button', { name: 'Held (1)' })).toBeVisible()
})
