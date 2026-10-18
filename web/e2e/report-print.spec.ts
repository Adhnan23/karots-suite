import { expect, test } from './fixtures'

// A paged report prints every row, not only the 50 on screen: Print loads the rest
// first (a PDF from the print dialog had only the first page of rows).
test('Print loads every sale before printing, so the PDF lists them all', async ({ page }) => {
  test.setTimeout(120_000)
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(admin|till)/)
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 4 }, sourceLockerId: 0 } })
  const milk = (await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()).id
  for (let i = 0; i < 55; i++) {
    await page.request.post('/api/v1/till/sales', { data: { saleType: 'retail', discountType: 'fixed', payments: [{ method: 'cash', amount: '5000' }],
      items: [{ productId: milk, quantity: '1', discountType: 'fixed' }] } })
  }
  await page.addInitScript(() => { window.print = () => { document.body.dataset.printed = '1' } })
  await page.goto('/admin/reports/sales?preset=today')
  const shown = page.getByText(/shown · tap one for its lines/)
  await expect(shown).toHaveText(/^50\+ shown/)
  await page.getByRole('button', { name: 'Print' }).click()
  await expect(page.locator('body[data-printed="1"]')).toBeAttached()
  await expect(shown).toHaveText(/^(5[5-9]|[6-9]\d|\d{3,}) shown/) // every sale, no "+" left
  await page.emulateMedia({ media: 'print' })
  const pdf = (await page.pdf({ format: 'A4' })).toString('latin1')
  expect((pdf.match(/\/Type\s*\/Page[^s]/g) ?? []).length).toBeGreaterThan(2)
})
