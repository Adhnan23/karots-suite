import { expect, test, type Page } from './fixtures'

// Extra barcodes (M7.6a). Needs `make e2e-seed`. A run tag keeps codes unique on re-runs.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString().slice(-7)
const NEW_CODE = `600${tag}1` // arrives at intake, really E2E Milk
const RECV_CODE = `601${tag}2` // arrives on a delivery, really E2E Lot Soap

async function signIn(page: Page, phone: string, path: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(path)
  await page.waitForLoadState('networkidle')
}
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()
async function pick(page: Page, name: string) {
  await page.getByLabel('Find a product').fill(name)
  await page.getByRole('button', { name: new RegExp(`^${name}\\b`) }).first().click()
}

test('intake: an unknown scan joins an existing product, and the till sells it by that code', async ({ page }) => {
  await signIn(page, '0770000003', '/admin/inventory/intake')
  await page.getByLabel('Scan or search').fill(NEW_CODE)
  await page.getByLabel('Scan or search').press('Enter')
  await page.getByRole('button', { name: 'Same as an existing one' }).click()
  await pick(page, 'E2E Milk')
  await toast(page, new RegExp(`${NEW_CODE} added to E2E Milk`))
  await expect(page.getByText('E2E Milk').first()).toBeVisible() // its restock panel
  // The till finds it by the new code.
  const res = await page.request.get(`/api/v1/till/products/barcode/${NEW_CODE}`)
  expect((await res.json()).name).toBe('E2E Milk')
})

test('product form: the extra shows its last scan; make main, back again, then remove', async ({ page }) => {
  await signIn(page, '0770000003', '/admin/products')
  await page.getByLabel('Search products').fill('E2E Milk')
  await page.getByRole('button', { name: /^E2E Milk\b/ }).first().click()
  const block = page.getByRole('region', { name: 'Other barcodes' })
  const row = block.getByRole('listitem').filter({ hasText: NEW_CODE })
  await expect(row).toContainText('scanned today')
  await row.getByRole('button', { name: 'Make main' }).click()
  await expect(page.locator('input[name="prod-code"]')).toHaveValue(NEW_CODE)
  await block.getByRole('listitem').filter({ hasText: '4790000000011' }).getByRole('button', { name: 'Make main' }).click()
  await expect(page.locator('input[name="prod-code"]')).toHaveValue('4790000000011')
  await block.getByRole('listitem').filter({ hasText: NEW_CODE }).getByRole('button', { name: /Remove/ }).click()
  await page.getByRole('button', { name: 'Remove', exact: true }).click()
  await expect(block.getByText(NEW_CODE)).toHaveCount(0)
})

test('purchase entry: an unknown scan joins an existing product as a line', async ({ page }) => {
  await signIn(page, '0770000003', '/admin/purchases/new')
  await page.getByLabel('Add a product').fill(RECV_CODE)
  await page.getByLabel('Add a product').press('Enter')
  await page.getByRole('button', { name: 'Same as an existing one' }).click()
  await pick(page, 'E2E Lot Soap')
  await expect(page.getByLabel('Quantity of E2E Lot Soap').or(page.getByText('E2E Lot Soap')).first()).toBeVisible()
  const res = await page.request.get(`/api/v1/till/products/barcode/${RECV_CODE}`)
  expect((await res.json()).name).toBe('E2E Lot Soap')
})
