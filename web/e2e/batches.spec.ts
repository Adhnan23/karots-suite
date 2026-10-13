import { expect, freshProduct, receiveExpired, test, type Page } from './fixtures'

// M7.6b batches: add / merge on the Batches sheet, stock-take batch by batch,
// intake into the matching batch, a write-off settled with the supplier.
// Needs `make e2e-seed`.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString(36).slice(-5)
const name = `E2E Batchy ${tag}`
let pid = 0

async function signIn(page: Page, phone = '0770000003') {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}
type Lot = { id: number; qtyRemaining: string; costPrice: string; sellingPrice: string; expiryDate: string }
const lots = async (page: Page) => (await (await page.request.get(`/api/v1/admin/stock/lots/${pid}`)).json()) as { stockQty: string; lots: Lot[] }
async function openBatches(page: Page) {
  await page.goto(`/admin/stock?q=${encodeURIComponent(name)}`)
  await page.getByText(name).locator('visible=true').first().click()
  await expect(page.getByRole('dialog')).toContainText(`Batches of ${name}`)
}

test('batches: add two batches, merge them at an edited cost — the count stays', async ({ page }) => {
  await signIn(page)
  pid = await freshProduct(page, name)
  await openBatches(page)
  for (const [qty, cost, exp] of [['3', '40', ''], ['2', '50', '2027-01-31']]) {
    await page.getByRole('button', { name: '+ Add batch' }).click()
    await page.getByLabel('Quantity').fill(qty)
    await page.getByLabel('Cost').fill(cost)
    if (exp) await page.getByLabel('Expiry').fill(exp)
    await page.getByRole('button', { name: 'Add batch' }).click()
    await expect(page.getByRole('button', { name: '+ Add batch' })).toBeVisible()
  }
  await page.getByRole('button', { name: 'Merge batches' }).click()
  await page.getByLabel('Merge batch 1').check()
  await page.getByLabel('Merge batch 2').check()
  await expect(page.getByLabel('Merged cost')).toHaveValue('44.00') // (3×40 + 2×50) / 5
  await page.getByLabel('Merged cost').fill('45')
  await expect(page.getByRole('group', { name: 'Merge batches' })).toContainText('Stock value +Rs. 5.00')
  await page.getByRole('button', { name: 'Merge', exact: true }).click()
  await expect.poll(async () => (await lots(page)).lots.map((l) => [l.qtyRemaining, l.costPrice, l.expiryDate])).toEqual([['5', '45.00', '2027-01-31']])
})

test('stock-take: count a product batch by batch', async ({ page }) => {
  await signIn(page)
  await page.request.post(`/api/v1/admin/stock/lots/${pid}/add`, { data: { qty: '2', cost: '', sellingPrice: '', expiry: '' } })
  await page.goto(`/admin/stock/take?q=${encodeURIComponent(name)}`)
  await page.getByRole('button', { name: /count its 2 batches/ }).click()
  await expect(page.getByLabel(`Counted ${name}`, { exact: true })).toBeDisabled()
  await page.getByLabel(`Counted ${name} batch 1`).fill('4')
  await page.getByRole('button', { name: 'Save counts' }).click()
  await expect.poll(async () => (await lots(page)).stockQty).toBe('6')
})

test('intake: a restock tops up the batch that matches, not a new one', async ({ page }) => {
  await signIn(page)
  const before = (await lots(page)).lots.length
  await page.goto('/admin/inventory/intake')
  await page.getByRole('textbox').first().fill(name)
  await page.getByRole('button', { name: new RegExp(name) }).first().click()
  // Cost 40 / price 60 typed = the product's: the batch added at the product's cost matches.
  await expect(page.getByRole('radio', { checked: true }).locator('xpath=ancestor::label')).toContainText('matches')
  await page.getByLabel('Quantity to add').fill('1')
  await page.getByLabel('Print labels on save').uncheck().catch(() => {})
  await page.getByRole('button', { name: /^Add stock/ }).click()
  await expect.poll(async () => (await lots(page)).stockQty).toBe('7')
  expect((await lots(page)).lots.length).toBe(before)
})

test('counter: a supplier’s expired stock is settled where they’re received — replaced with an extra', async ({ page }) => {
  await signIn(page)
  await receiveExpired(page, pid, '2')
  const sup = (await (await page.request.get('/api/v1/till/suppliers?q=E2E%20Lot%20Source')).json())[0]
  const owed = async () => Number((await (await page.request.get(`/api/v1/till/suppliers/${sup.id}`)).json()).owed)
  const before = await owed()
  await page.goto('/till/suppliers')
  await page.getByText('E2E Lot Source').first().click()
  await page.getByRole('button', { name: /to settle with them/ }).click()
  const panel = page.getByRole('region', { name: 'From this supplier' })
  await panel.getByRole('listitem').filter({ hasText: name }).getByRole('button', { name: 'Write off & settle' }).click()
  const sheet = page.getByRole('dialog', { name: `Recover: ${name}` })
  await sheet.getByLabel('Extra per unit').fill('5')
  await sheet.getByLabel('Expiry', { exact: true }).fill('2027-12-31')
  await sheet.getByRole('button', { name: 'Record' }).click()
  await expect.poll(owed).toBe(before + 10) // 2 × Rs 5, owed (not paid now)
  await expect.poll(async () => (await lots(page)).lots.some((l) => l.expiryDate === '2027-12-31' && l.costPrice === '45.00' && l.qtyRemaining === '2')).toBe(true)
  await expect(panel.getByRole('listitem').filter({ hasText: name })).toHaveCount(0)
})
