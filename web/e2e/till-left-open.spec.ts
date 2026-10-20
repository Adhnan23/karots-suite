import { expect, test, type Page } from './fixtures'

// A till left open: the cashier must close it before selling on; an admin can close it for them.
// Its own cashier each run, so no other spec's drawer moves.
test.setTimeout(90_000)

async function signIn(page: Page, phone: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}

test('a till from yesterday must be closed and opened again; an admin closes one for them', async ({ page, browser }) => {
  await signIn(page, '0770000001')
  const phone = `071${String(Date.now()).slice(-7)}`
  const res = await page.request.post('/api/v1/admin/users', { data: { name: `Left Open ${phone.slice(-4)}`, phone, role: 'cashier', pin: '1234', receiptPrinter: '', canHandleSuppliers: false, canManageCredit: false, canSeeCost: false } })
  expect(res.ok()).toBeTruthy()
  const ctx = await browser.newContext()
  const cashier = await ctx.newPage()
  await signIn(cashier, phone)
  await cashier.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })

  // The server says this till is from an earlier day (a real one needs yesterday's clock).
  await cashier.route('**/api/v1/till/drawer', async (route) => {
    const r = await route.fetch()
    const body = await r.json()
    if (body.summary.session) body.summary.stale = true
    await route.fulfill({ response: r, json: body })
  })
  await cashier.goto('/till')
  const dlg = cashier.getByRole('dialog', { name: 'Close the till' })
  await expect(dlg).toContainText('was never closed')
  await expect(dlg.getByRole('button', { name: 'Cancel' })).toHaveCount(0)
  await cashier.keyboard.press('Escape')
  await expect(dlg).toBeVisible() // can't be put off
  await dlg.getByLabel(/Count of .*500\.00/).fill('2')
  await dlg.getByRole('button', { name: /Close & reconcile/ }).click()
  await cashier.getByRole('button', { name: 'Done' }).click()
  await expect(cashier.getByRole('dialog', { name: 'Open the till' })).toBeVisible()

  // Opened again, then left: the admin closes it for them from Till sessions.
  await cashier.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
  const open = await (await page.request.get('/api/v1/admin/tills?open=1')).json()
  const mine = open.find((r: { cashier: string }) => r.cashier === `Left Open ${phone.slice(-4)}`)
  // Untracked cash off: the cash must be banked into the locker shown (picked once the lockers load).
  const cur = (await (await page.request.get('/api/v1/admin/settings')).json()).settings
  const off = await page.request.put('/api/v1/admin/settings', { data: { ...cur, allowUntrackedCash: false } })
  expect(off.ok(), await off.text()).toBeTruthy()
  try {
    await page.goto(`/admin/tills/${mine.id}`)
    await page.getByRole('button', { name: 'Close for them' }).click()
    const close = page.getByRole('dialog', { name: /till for them/ })
    await close.getByLabel(/Count of .*500\.00/).fill('2')
    await close.getByRole('button', { name: /Close & reconcile/ }).click()
    await expect(page.getByRole('dialog', { name: /till closed/ })).toContainText('Over / short')
    await page.getByRole('button', { name: 'Done' }).click()
  } finally {
    await page.request.put('/api/v1/admin/settings', { data: cur })
  }
  await expect(page.getByRole('heading', { level: 1 })).toContainText('closed by E2E Admin')
  await expect(page.getByRole('button', { name: 'Close for them' })).toHaveCount(0)
  await ctx.close()
})
