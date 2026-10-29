import { expect, test, type Page } from './fixtures'

// Customer requests (plugin, built in for Playwright). Needs `make e2e-seed`. Serial: one drawer.
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const umbrella = `Req umbrella ${tag}`
const nomatch = `zzq bracket ${tag}`
const torch = `Req torch ${tag}`

async function login(page: Page, phone: string, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}

async function till(page: Page) {
  await login(page, '0770000002', '/till')
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) {
    await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
    await page.reload()
  }
}

const note = (page: Page) => page.getByRole('textbox', { name: 'What did they ask for?' })
const row = (page: Page, text: string) => page.getByRole('table', { name: 'Customer requests' }).getByRole('row').filter({ hasText: text })
const menu = async (page: Page, text: string, item: string) => {
  await row(page, text).getByRole('button', { name: /Actions for/ }).click()
  await page.getByRole('menuitem', { name: item }).click()
}

test('the till notes a request, and a second customer asks again instead of a duplicate', async ({ page }) => {
  await till(page)
  await page.getByRole('button', { name: 'Note request' }).click()
  await note(page).fill(umbrella)
  await page.getByLabel('Customer name').fill('Nimal')
  await page.getByLabel('Phone').fill('0771234567')
  await page.getByRole('button', { name: 'Save request' }).click()
  await expect(page.getByText('Request noted')).toBeVisible()

  await page.getByRole('button', { name: 'Note request' }).click()
  await note(page).fill(`umbrella ${tag}`)
  await expect(page.getByText('Asked before')).toBeVisible()
  await page.getByLabel('Customer name').fill('Kamala')
  await page.getByRole('button', { name: 'Asked again' }).click()
  await expect(page.getByText(`Added to “${umbrella}”`)).toBeVisible()
})

test('a search that finds nothing offers to note a request with the text', async ({ page }) => {
  await till(page)
  const s = page.getByLabel('Scan a barcode or search products')
  await s.fill(nomatch)
  await s.press('Enter')
  await page.getByRole('button', { name: 'Not stocked? Note a request' }).click()
  await expect(note(page)).toHaveValue(nomatch)
  await page.getByRole('button', { name: 'Save request' }).click()
  await expect(page.getByText('Request noted')).toBeVisible()
})

test('admin makes a product; once stocked the customer is on To call, then Done', async ({ page }) => {
  await login(page, '0770000001', '/admin')
  await page.getByRole('button', { name: 'Apps' }).click()
  await page.getByRole('link', { name: /Customer requests/ }).click()
  await expect(page).toHaveURL(/\/admin\/x\/requests$/)
  await expect(row(page, umbrella)).toContainText('Asked 2×')
  await expect(row(page, umbrella)).toContainText('Kamala')
  await expect(row(page, nomatch)).toBeVisible()

  await menu(page, umbrella, 'Make product')
  await page.getByRole('textbox', { name: 'Product name' }).fill(`Umbrella red ${tag}`)
  await page.getByRole('button', { name: 'Category' }).click()
  await page.getByLabel('Search categories').last().fill('E2E Goods')
  await page.getByRole('list', { name: 'Categories' }).last().getByRole('button', { name: /E2E Goods/ }).first().click()
  await page.getByLabel('Selling price').fill('1500')
  await page.getByRole('button', { name: 'Create product' }).click()
  await expect(row(page, umbrella)).toContainText('Waiting for stock')

  const pid = (await (await page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(`Umbrella red ${tag}`)}`)).json()).items[0].id
  await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: pid, newQuantity: '3', note: 'e2e', batchId: 0 } })
  await page.getByRole('button', { name: /^To call/ }).click()
  await expect(row(page, umbrella)).toContainText('In stock')
  await expect(row(page, umbrella).getByRole('link', { name: '0771234567' })).toHaveAttribute('href', 'tel:0771234567')
  await row(page, umbrella).getByRole('button', { name: 'Called ✓' }).click()
  await expect(row(page, umbrella)).toHaveCount(0)
  await page.getByRole('button', { name: /^Done/ }).click()
  await expect(row(page, umbrella)).toContainText('called')
})

test("won't stock is kept and the till says why; merge folds asks together", async ({ page }) => {
  await login(page, '0770000001', '/admin/x/requests')
  const add = (n: string) => page.request.post('/api/v1/x/requests', { data: { note: n, name: '', phone: '' } })
  await add(torch)
  await add(`${torch} rechargeable`)
  await page.reload()

  await menu(page, `${torch} rechargeable`, 'Merge into…')
  await page.getByRole('button', { name: new RegExp(`^${torch} Asked`) }).click()
  await expect(page.getByRole('dialog')).toHaveCount(0) // the table is hidden from roles while it's open
  await expect(row(page, `${torch} rechargeable`)).toHaveCount(0)
  await expect(row(page, torch)).toContainText('Asked 2×')

  // Edit: the note and each customer's name / phone.
  await menu(page, torch, 'Edit')
  await page.getByLabel('Name of customer 1').fill('Ruwan')
  await page.getByLabel('Phone of customer 1').fill('0711112222')
  await page.getByRole('button', { name: 'Remove customer 2' }).click()
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(row(page, torch)).toContainText('Ruwan')
  await expect(row(page, torch)).toContainText('Asked today')

  await menu(page, torch, 'Won’t stock')
  await page.getByLabel('Why not?').fill('discontinued')
  await page.getByRole('button', { name: 'Save' }).click()
  await page.getByRole('button', { name: /^Done/ }).click()
  await expect(row(page, torch)).toContainText("Won't stock: discontinued")

  await till(page)
  await page.getByRole('button', { name: 'Note request' }).click()
  await note(page).fill(torch)
  await expect(page.getByText('Won’t stock: discontinued')).toBeVisible()
  // Leave the drawer as the till specs expect to find it: closed.
  await page.request.post('/api/v1/till/drawer/close', { data: { breakdown: {}, destLockerId: 0 } })
})

test('the page fits a phone', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 })
  await login(page, '0770000001', '/admin/x/requests')
  await expect(page.getByRole('list', { name: 'Customer requests' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
})
