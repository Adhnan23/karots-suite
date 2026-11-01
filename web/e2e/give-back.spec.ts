import { expect, freshProduct, test, type Page } from './fixtures'

// A lot goes back to its supplier: told them, counted, waiting, settled.
// Needs `make e2e-seed`.
const tag = Date.now().toString(36).slice(-5)
const name = `E2E Giving ${tag}`
const day = (n: number) => new Date(Date.now() + n * 86_400_000).toISOString().slice(0, 10)

async function signIn(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}

test('close to expiry: told them, counted more than the system, waits, settled with money', async ({ page }) => {
  await signIn(page)
  const sup = await (await page.request.post('/api/v1/admin/suppliers', { data: { name: `E2E Taker ${tag}`, contact: '', phone: '', address: '', creditDays: 30, openingBalance: '' } })).json()
  const pid = await freshProduct(page, name)
  const receive = async (qty: string, expiry: string) => {
    const res = await page.request.post(`/api/v1/till/suppliers/${sup.id}/receive`, { data: {
      invoiceNo: '', discount: '', discountType: 'fixed', notes: '', keepRemainder: false, deferDrawerKick: true,
      pay: { amount: '', method: 'cash', lockerId: 0 },
      lines: [{ productId: pid, name: '', ordered: '', quantity: qty, freeQty: '', costPrice: '40', sellingPrice: '', expiryDate: expiry, discount: '', discountType: 'fixed' }],
    } })
    expect(res.ok()).toBeTruthy()
  }
  await receive('2', day(20))
  await receive('5', day(300))
  const lots = async () => ((await (await page.request.get(`/api/v1/admin/stock/lots/${pid}`)).json()).lots as { qtyRemaining: string; expiryDate: string }[]).map((l) => [l.qtyRemaining, l.expiryDate])
  const owed = async () => Number((await (await page.request.get(`/api/v1/admin/suppliers/${sup.id}`)).json()).owed)

  // The supplier page shows it, and remembers they were told.
  await page.goto(`/admin/suppliers/${sup.id}`)
  await page.getByRole('button', { name: /to settle with them/ }).click()
  const panel = page.getByRole('region', { name: 'From this supplier' })
  const closeRow = panel.getByRole('listitem').filter({ hasText: name })
  await expect(closeRow).toContainText('expires')
  await closeRow.getByRole('button', { name: 'Told them' }).click()
  await expect(closeRow).toContainText('told')

  // Give back from Batches: 3 counted, 2 in the system. The other lot gives one up.
  await page.goto('/admin/stock/batches')
  const group = page.getByRole('list', { name: 'Products with lots' }).getByRole('listitem').filter({ hasText: name }).first()
  await expect(group).toContainText('told')
  await group.getByRole('button', { name: 'Give back' }).first().click()
  const count = page.getByRole('dialog', { name: `Give back: ${name}` })
  await count.getByLabel('Counted on the shelf').fill('3')
  await expect(count).toContainText('1 more than the system')
  await count.getByRole('button', { name: 'Give back' }).click()
  await page.getByRole('button', { name: 'Later' }).click()
  await expect.poll(lots).toEqual([['4', day(300)]])

  // It waits on them, and money settles it.
  const before = await owed()
  await page.goto(`/admin/suppliers/${sup.id}`)
  await page.getByRole('button', { name: /to settle with them/ }).click()
  const waiting = panel.getByRole('listitem').filter({ hasText: name })
  await expect(waiting).toContainText('3 given back')
  await waiting.getByRole('button', { name: 'Settle' }).click()
  const sheet = page.getByRole('dialog', { name: `Recover: ${name}` })
  await sheet.getByRole('radio', { name: 'Credited us' }).or(sheet.getByRole('button', { name: 'Credited us' })).first().click()
  await sheet.getByLabel('Amount credited').fill('120')
  await sheet.getByRole('button', { name: 'Record' }).click()
  await expect.poll(owed).toBe(before - 120)
  await expect(panel).toHaveCount(0)
})

test('recall from Batches: stop selling, listed on the supplier, undo', async ({ page }) => {
  await signIn(page)
  const rtag = Date.now().toString(36).slice(-5)
  const rname = `E2E Recalled ${rtag}`
  const sup = await (await page.request.post('/api/v1/admin/suppliers', { data: { name: `E2E Recaller ${rtag}`, contact: '', phone: '', address: '', creditDays: 0, openingBalance: '' } })).json()
  const pid = await freshProduct(page, rname)
  const res = await page.request.post(`/api/v1/till/suppliers/${sup.id}/receive`, { data: {
    invoiceNo: '', discount: '', discountType: 'fixed', notes: '', keepRemainder: false, deferDrawerKick: true,
    pay: { amount: '', method: 'cash', lockerId: 0 },
    lines: [{ productId: pid, name: '', ordered: '', quantity: '4', freeQty: '', costPrice: '40', sellingPrice: '', expiryDate: day(400), discount: '', discountType: 'fixed' }],
  } })
  expect(res.ok()).toBeTruthy()

  await page.goto('/admin/stock/batches')
  const group = page.getByRole('list', { name: 'Products with lots' }).getByRole('listitem').filter({ hasText: rname }).first()
  await group.getByRole('button', { expanded: false }).click() // nowhere near expiry, so it starts closed
  await group.getByRole('button', { name: 'Recall', exact: true }).click()
  await page.getByRole('button', { name: 'Stop selling' }).click()
  await expect(group).toContainText('recalled, not selling')

  await page.goto(`/admin/suppliers/${sup.id}`)
  await page.getByRole('button', { name: /to settle with them/ }).click()
  const panel = page.getByRole('region', { name: 'From this supplier' })
  const row = panel.getByRole('listitem').filter({ hasText: rname })
  await expect(row).toContainText('not selling')
  await row.getByRole('button', { name: 'Undo recall' }).click()
  await expect(panel).toHaveCount(0)
})

test('a discount at payment closes the bill, moves no money and shows as a gain', async ({ page }) => {
  await signIn(page)
  const dtag = Date.now().toString(36).slice(-5)
  const sup = await (await page.request.post('/api/v1/admin/suppliers', { data: { name: `E2E Discounter ${dtag}`, contact: '', phone: '', address: '', creditDays: 0, openingBalance: '' } })).json()
  const pid = await freshProduct(page, `E2E Discounted ${dtag}`)
  const res = await page.request.post(`/api/v1/till/suppliers/${sup.id}/receive`, { data: {
    invoiceNo: '', discount: '', discountType: 'fixed', notes: '', keepRemainder: false, deferDrawerKick: true,
    pay: { amount: '', method: 'cash', lockerId: 0 },
    lines: [{ productId: pid, name: '', ordered: '', quantity: '1', freeQty: '', costPrice: '1000', sellingPrice: '', expiryDate: '', discount: '', discountType: 'fixed' }],
  } })
  expect(res.ok()).toBeTruthy()
  const owed = async () => Number((await (await page.request.get(`/api/v1/admin/suppliers/${sup.id}`)).json()).owed)

  await page.goto(`/admin/suppliers/${sup.id}`)
  await page.getByRole('button', { name: '↑ Pay' }).click()
  const sheet = page.getByRole('dialog', { name: `Pay E2E Discounter ${dtag}` })
  await sheet.getByRole('radio', { name: 'Discount' }).or(sheet.getByRole('button', { name: 'Discount', exact: true })).first().click()
  await expect(sheet).toContainText('no money moves')
  await sheet.getByLabel('Amount paid').fill('200')
  await sheet.getByRole('button', { name: /Record discount/ }).click()
  await expect.poll(owed).toBe(800)
  await expect(page.getByText('Discount received').first()).toBeVisible()
  await expect(page.getByText('no money moved').first()).toBeVisible()

  await page.goto('/admin/finance')
  await expect(page.getByText('Discounts from suppliers')).toBeVisible()
})
