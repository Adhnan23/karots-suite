import { expect, test } from './fixtures'

// A cashier PC's print helper calls in; the admin approves it and picks its printer.
test('a cashier PC asks, is approved and gets its printer', async ({ page, request }) => {
  const token = `e2e-print-helper-token-${Date.now()}-000000000000`
  const poll = () => request.post('/api/v1/print-agent/poll', { headers: { 'X-Ks-Print-Agent': token }, data: { name: 'Counter laptop', printers: ['XP80', 'XP365'] } })
  expect((await (await poll()).json()).approved).toBe(false)

  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto('/admin/settings')
  const card = page.getByRole('region', { name: "Cashier PCs' printers" })
  await expect(card.getByText('Waiting for your OK')).toBeVisible()
  await card.getByLabel('Counter laptop Receipt printer').selectOption('XP80')
  await card.getByRole('button', { name: 'Approve' }).click()
  await expect(card.getByText('Connected')).toBeVisible()
  await expect(card.getByLabel('Counter laptop Receipt printer')).toHaveValue('XP80')
  await page.screenshot({ path: 'shots/print-machines/settings.png', fullPage: true })

  // This test's browser shares the helper's address: forget the machine, so later specs print as before.
  await card.getByRole('button', { name: 'Remove' }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Remove' }).click()
  await expect(card).toHaveCount(0)
})
