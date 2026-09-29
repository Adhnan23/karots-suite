import { expect, test, type Page } from './fixtures'

// The rest of the M2 admin routes, so every ported route has a flow. Needs `make e2e-seed`.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString(36).slice(-5)

async function admin(page: Page, path: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000003')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(path)
  await page.waitForLoadState('networkidle')
}
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()

test('units: rename one', async ({ page }) => {
  await admin(page, '/admin/units')
  await page.request.post('/api/v1/admin/units', { data: { name: `Bag ${tag}`, abbreviation: `b${tag}`.slice(0, 10), allowDecimal: false } })
  await page.reload()
  await page.getByRole('button', { name: new RegExp(`^Bag ${tag}`) }).first().click()
  await page.getByLabel('Name').fill(`Sack ${tag}`)
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await toast(page, /Unit saved/)
  await expect(page.getByText(`Sack ${tag}`).first()).toBeVisible()
})

test('categories: delete an empty one', async ({ page }) => {
  await admin(page, '/admin/categories')
  await page.request.post('/api/v1/admin/categories', { data: { name: `Empty ${tag}`, parentId: 0 } })
  await page.reload()
  await page.getByRole('button', { name: new RegExp(`^Empty ${tag}`) }).first().click()
  await page.getByRole('button', { name: 'Delete', exact: true }).click()
  await page.getByRole('button', { name: 'Delete category' }).click()
  await toast(page, /Category deleted/)
})

test('products: quick barcode on a row, and a review item marked fine', async ({ page }) => {
  await admin(page, '/admin/products')
  await page.request.post('/api/v1/till/quick-item', { data: { name: `Nocode ${tag}`, price: '20', qty: '1', barcode: '', unitId: 0 } })
  await page.getByLabel('Search products').fill(`Nocode ${tag}`)
  await page.getByRole('button', { name: 'Add barcode' }).first().click()
  await page.getByRole('button', { name: 'Generate' }).click()
  await page.getByRole('button', { name: 'Save barcode' }).click()
  await toast(page, /Barcode 2\d{12} added/)

  await page.goto('/admin/products/review')
  await page.getByRole('row', { name: new RegExp(`Nocode ${tag}`) }).getByRole('button', { name: 'Looks fine' }).click()
  await page.getByRole('button', { name: 'Mark reviewed' }).click()
  await toast(page, /Marked reviewed/)
  await expect(page.getByText(`Nocode ${tag}`)).toHaveCount(0)
})

test('cashier menu: reorder cards and take a product off one', async ({ page }) => {
  await admin(page, '/admin/groups')
  const mk = async (name: string) => (await (await page.request.post('/api/v1/admin/groups', { data: { name, emoji: '', parentId: 0 } })).json()).id
  await mk(`A ${tag}`)
  const b = await mk(`B ${tag}`)
  const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
  await page.request.post(`/api/v1/admin/groups/${b}/items`, { data: { productId: milk.id, emoji: '' } })
  await page.reload()
  await page.getByRole('button', { name: `Move B ${tag} up` }).click()
  await expect.poll(async () => {
    const names = await page.getByRole('list', { name: 'Groups' }).getByRole('button', { pressed: false }).allTextContents()
    return names.findIndex((n) => n.includes(`B ${tag}`)) < names.findIndex((n) => n.includes(`A ${tag}`))
  }).toBeTruthy()
  await page.getByRole('button', { name: new RegExp(`^📦 B ${tag}`) }).click()
  await page.getByRole('button', { name: 'Take E2E Milk off this card' }).click()
  await expect(page.getByText('No products on this card yet.')).toBeVisible()
})

test('conversions: change the ratio, disable, bring back', async ({ page }) => {
  await admin(page, '/admin/conversions')
  const row = page.getByRole('row', { name: /E2E Milk Crate/ })
  await row.getByRole('button', { name: 'Edit' }).click()
  await page.getByLabel('Ratio').fill('10')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await toast(page, /Conversion saved/)
  await expect(row.getByText('1 pcs = 10 pcs')).toBeVisible()
  await row.getByRole('button', { name: 'Edit' }).click()
  await page.getByRole('button', { name: 'Disable', exact: true }).click()
  await page.getByRole('button', { name: 'Disable', exact: true }).last().click()
  await toast(page, /Conversion disabled/)
  await page.getByRole('button', { name: 'Show disabled' }).click()
  await page.getByRole('row', { name: /E2E Milk Crate/ }).getByRole('button', { name: 'Edit' }).click()
  await page.getByRole('button', { name: 'Enable' }).click()
  await toast(page, /Conversion enabled/)
  await page.request.put(`/api/v1/admin/conversions/${(await (await page.request.get('/api/v1/admin/conversions?q=Crate')).json())[0].id}`, { data: { fromProductId: 0, toProductId: 0, ratio: '12', note: '' } })
})

test('lots: price one lot, then let it follow the product again', async ({ page }) => {
  await admin(page, '/admin/stock')
  await page.getByLabel('Search products').fill('4790000000011')
  await page.getByRole('button', { name: /^E2E Milk/ }).first().click()
  const price = page.getByLabel('Price of lot 1')
  await price.fill('95')
  await price.press('Enter')
  await toast(page, /Lot priced at Rs\. 95\.00/)
  await price.fill('')
  await price.press('Enter')
  await toast(page, /follows the product price again/)
})

test('batches: write off every expired lot at once', async ({ page }) => {
  await admin(page, '/admin/stock/batches')
  const expiredBefore = (await (await page.request.get('/api/v1/admin/stock/batches')).json()).expired
  test.skip(expiredBefore === 0, 'no expired lot left in this database — reseed')
  await page.getByRole('button', { name: 'Write off all' }).click()
  await page.getByRole('button', { name: 'Write off', exact: true }).last().click()
  await toast(page, /expired lots? written off/)
  expect((await (await page.request.get('/api/v1/admin/stock/batches')).json()).expired).toBe(0)
})

test('stock-take: download the count sheet, fill it, import it', async ({ page }) => {
  await admin(page, '/admin/stock/take')
  const sheet = await (await page.request.get('/api/v1/admin/stock/take/sheet?format=csv')).text()
  const milkLine = sheet.split('\n').find((l) => l.startsWith('4790000000011'))!
  const filled = `${sheet.split('\n')[0]}\n${milkLine.replace(/^([^,]*,[^,]*,[^,]*),,/, '$1,333,')}\n`
  await page.locator('input[type=file]').setInputFiles({ name: 'count.csv', mimeType: 'text/csv', buffer: Buffer.from(filled) })
  await expect(page.getByText('Count imported')).toBeVisible()
  await expect(page.getByText('Updated')).toBeVisible()
  expect(Number((await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()).stockQty)).toBe(333)
})
