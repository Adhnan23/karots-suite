import { expect, test } from './fixtures'
import type { Page } from '@playwright/test'

// A new batch often carries a new barcode. A cashier receiving stock is asked first,
// the choice works at once, and the admin can check it or take it back.
async function signIn(page: Page, phone: string, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}

test('a cashier links a new barcode on a delivery; the admin reviews it and can remove it', async ({ page, browser }) => {
  const code = `602${Date.now().toString().slice(-9)}`
  await signIn(page, '0770000005', '/till/suppliers')
  await page.getByRole('button', { name: /E2E Distributors/ }).click()
  await page.getByRole('button', { name: 'Receive stock' }).click()
  await expect(page.getByRole('button', { name: 'New product', exact: true })).toBeVisible()
  await page.getByLabel('Add a product').fill(code)
  await page.getByLabel('Add a product').press('Enter')
  await expect(page.getByLabel('New product name')).toHaveCount(0) // the question comes first
  await page.getByRole('button', { name: 'Same as an existing one' }).click()
  await page.getByLabel('Find the product').fill('E2E Lot Soap')
  await page.getByRole('button', { name: 'E2E Lot Soap' }).click()
  await expect(page.getByLabel('Quantity of E2E Lot Soap')).toBeVisible()
  expect((await (await page.request.get(`/api/v1/till/products/barcode/${code}`)).json()).name).toBe('E2E Lot Soap')

  const admin = await (await browser.newContext({ viewport: { width: 360, height: 780 } })).newPage()
  await signIn(admin, '0770000001', '/admin/products/review')
  await expect(admin.getByText(`Barcode ${code} linked here by`).locator('visible=true')).toBeVisible() // a table and cards: one shows
  await admin.screenshot({ path: 'shots/barcode-review/phone.png' })
  await admin.getByRole('button', { name: `Remove ${code}` }).click()
  await admin.getByRole('dialog').getByRole('button', { name: 'Remove barcode' }).click()
  await expect(admin.getByText(`Barcode ${code} linked here by`)).toHaveCount(0)
  expect((await page.request.get(`/api/v1/till/products/barcode/${code}`)).status()).toBe(404)
})
