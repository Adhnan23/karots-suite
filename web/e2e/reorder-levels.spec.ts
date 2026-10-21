import { expect, test, type Page } from './fixtures'

// Smart reorder levels on the admin side (docs/specs/2026-10-21-smart-reorder-levels-design.md).
test.setTimeout(90_000)

async function signIn(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}

async function openProduct(page: Page, name: string) {
  await page.goto(`/admin/products?q=${encodeURIComponent(name)}`)
  await page.getByText(name, { exact: true }).locator('visible=true').first().click()
  return page.getByRole('radiogroup', { name: 'Reorder level mode' })
}

test('a new product is Auto; a Fixed one switched to Auto shows its learned level and why', async ({ page }) => {
  await signIn(page)
  const name = `E2E Reorder ${Date.now()}`
  const cats = await (await page.request.get('/api/v1/admin/categories')).json()
  const cat = (cats.items ?? cats).find((c: { name: string }) => c.name === 'E2E Goods')
  const made = await page.request.post('/api/v1/admin/products', { data: { name, nameLocal: '', barcode: '', categoryId: cat.id, unitId: 1, costPrice: '10', sellingPrice: '15', wholesalePrice: '', taxRate: '', reorderLevel: 4, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '' } })
  expect(made.ok(), await made.text()).toBeTruthy()

  const mode = await openProduct(page, name)
  await expect(mode.getByRole('radio', { name: 'Auto' })).toHaveAttribute('aria-checked', 'true')
  await expect(page.getByText('Updated every night.')).toBeVisible()

  // To Fixed and save: the number box is back.
  await mode.getByRole('radio', { name: 'Fixed' }).click()
  await page.getByLabel('Reorder level', { exact: true }).fill('9')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('radiogroup', { name: 'Reorder level mode' })).toHaveCount(0)
  const again = await openProduct(page, name)
  await expect(again.getByRole('radio', { name: 'Fixed' })).toHaveAttribute('aria-checked', 'true')
  await expect(page.getByLabel('Reorder level', { exact: true })).toHaveValue('9')
})

test('Levels look off lists a Fixed level far from its sales; Use fixes it', async ({ page }) => {
  await signIn(page)
  // Saving it learns its numbers now: 9 months old, never sold → sales suggest 0, not 20.
  await openProduct(page, 'E2E Shelf Fan')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('radiogroup', { name: 'Reorder level mode' })).toHaveCount(0)

  await page.goto('/admin/reorder')
  await page.getByRole('button', { name: /Levels look off · \d+/ }).click()
  const list = page.getByRole('list', { name: 'Levels look off' })
  const row = list.getByRole('listitem').filter({ hasText: 'E2E Shelf Fan' })
  await expect(row).toContainText('Fixed 20 → sales suggest 0')
  await row.getByRole('button', { name: 'Use 0' }).click()
  await expect(list.getByText('E2E Shelf Fan')).toHaveCount(0)
})

test('Settings switches a category to Auto', async ({ page }) => {
  await signIn(page)
  const stamp = String(Date.now()).slice(-7)
  const cat = await (await page.request.post('/api/v1/admin/categories', { data: { name: `E2E Switch ${stamp}`, parentId: 0 } })).json()
  const made = await page.request.post('/api/v1/admin/products', { data: { name: `E2E Switch item ${stamp}`, nameLocal: '', barcode: '', categoryId: cat.id, unitId: 1, costPrice: '10', sellingPrice: '15', wholesalePrice: '', taxRate: '', reorderLevel: 3, reorderAuto: false, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '' } })
  const { id } = await made.json()

  await page.goto('/admin/settings')
  await page.getByRole('button', { name: 'Category to switch' }).click()
  await page.getByLabel('Search categories').fill(`E2E Switch ${stamp}`)
  await page.getByLabel('Search categories').press('Enter')
  await page.getByRole('button', { name: 'Switch Fixed products to Auto' }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Switch to Auto' }).click()
  await expect(page.getByText('1 product switched to Auto')).toBeVisible()
  const p = await (await page.request.get(`/api/v1/admin/products/${id}`)).json()
  expect(p.reorderAuto).toBe(true)
  expect(p.reorderStart).toBe(3)
})
