import { expect, freshProduct, receiveExpired, test, type Page } from './fixtures'

// Golden admin stock flows (M2). Needs `make e2e-seed`.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString(36).slice(-5)

async function signIn(page: Page, phone = '0770000003') {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}
async function admin(page: Page, path: string) {
  await signIn(page)
  await page.goto(path)
  await page.waitForLoadState('networkidle')
}
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()
const product = async (page: Page, code: string) => (await page.request.get(`/api/v1/till/products/barcode/${code}`)).json()
const qty = async (page: Page, code: string) => Number((await product(page, code)).stockQty)

test('stock: adjust a count down from the lot that’s short, and see it in movements', async ({ page }) => {
  await admin(page, '/admin/stock')
  const before = await qty(page, '4790000000028')
  await page.getByLabel('Search products').fill('4790000000028')
  await page.getByRole('button', { name: 'Adjust' }).first().click()
  await page.getByLabel('Actually there').fill(String(before - 1))
  await page.getByRole('radio', { name: /at Rs\. 120\.00/ }).check()
  await page.getByRole('button', { name: 'Missing' }).click()
  await page.getByRole('button', { name: 'Save count' }).click()
  await toast(page, /E2E Lot Soap/)
  expect(await qty(page, '4790000000028')).toBe(before - 1)
  await page.getByRole('button', { name: 'Movements' }).click()
  await expect(page.getByRole('table', { name: 'Stock movements' }).getByText('Missing').first()).toBeVisible()
})

test('batches: write off the expired lot only', async ({ page }) => {
  await admin(page, '/admin/stock')
  const id = await freshProduct(page, `Curd ${tag}`)
  await receiveExpired(page, id, '3')
  await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: id, newQuantity: '5', note: 'fresh lot', batchId: 0 } }) // + 2 unexpired
  await page.goto('/admin/stock/batches')
  await page.getByLabel('Search products').fill(`Curd ${tag}`)
  await page.getByRole('button', { name: 'Write off', exact: true }).first().click()
  await page.getByRole('button', { name: 'Write off', exact: true }).last().click()
  await toast(page, /expired lot written off/)
  const lots = await (await page.request.get(`/api/v1/admin/stock/lots/${id}`)).json()
  expect(Number(lots.stockQty)).toBe(2) // only the good lot is left
})

test('stock-take: count two products, save once', async ({ page }) => {
  await admin(page, '/admin/stock/take')
  await page.getByLabel('Search products').fill('E2E Milk')
  await page.getByLabel('Counted E2E Milk', { exact: true }).fill('480')
  await page.getByLabel('Counted E2E Milk', { exact: true }).press('Enter')
  await expect(page.getByLabel('Counted E2E Milk Crate')).toBeFocused() // Enter walks down the list
  await page.keyboard.type('4')
  await page.getByRole('button', { name: 'Save counts' }).click()
  await toast(page, /2 products updated/)
  expect(await qty(page, '4790000000011')).toBe(480)
})

test('intake: create with an opening count, restock by scan, undo the create', async ({ page }) => {
  await admin(page, '/admin/inventory/intake')
  await page.getByLabel('Scan or search').fill(`Treacle ${tag}`)
  await page.getByLabel('Scan or search').press('Enter')
  await page.getByRole('button', { name: 'Category' }).click()
  await page.getByLabel('Search categories').fill('E2E Goods')
  await page.getByLabel('Search categories').press('Enter')
  await page.getByLabel('Cost').fill('500')
  await expect(page.getByLabel('Selling')).toHaveValue('700') // 1.4 default markup, whole rupees
  await page.getByLabel('Opening quantity').fill('6')
  await page.getByLabel('Print labels on save').uncheck()
  await page.getByRole('button', { name: 'Create & stock' }).click()
  await toast(page, new RegExp(`Treacle ${tag} added with 6`))
  await expect(page.getByLabel('Name')).toHaveValue('') // rapid loop: stays on New
  await expect(page.getByRole('button', { name: /Another like last/ })).toBeVisible()

  await page.getByLabel('Scan or search').fill('4790000000011')
  await page.getByLabel('Scan or search').press('Enter')
  const milk = await qty(page, '4790000000011')
  await page.getByLabel('Quantity to add').fill('12')
  await page.getByRole('button', { name: 'Add stock', exact: true }).click()
  await toast(page, /E2E Milk: \+12/)
  expect(await qty(page, '4790000000011')).toBe(milk + 12)

  const session = page.getByRole('region', { name: 'Added this session' })
  await session.getByRole('button', { name: `Undo Treacle ${tag}` }).click()
  await toast(page, /removed/)
  const found = await (await page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(`Treacle ${tag}`)}`)).json()
  expect(found.total).toBe(0) // disabled, recoverable under Show disabled
})

test('conversions: break a crate into loose milk', async ({ page }) => {
  await admin(page, '/admin/conversions')
  const crates = await qty(page, '4790000000059')
  await page.getByRole('button', { name: 'Run', exact: true }).first().click()
  await page.getByLabel('Packs to break').fill('1')
  await page.getByRole('button', { name: /^Make 12/ }).click()
  await toast(page, /1 pcs → 12 pcs/)
  expect(await qty(page, '4790000000059')).toBe(crates - 1)
  await page.getByRole('button', { name: 'Run history' }).click()
  await expect(page.getByRole('table', { name: 'Conversion runs' }).getByText('E2E Milk Crate → E2E Milk').first()).toBeVisible()
})

test('write-off: shop use from the stock page', async ({ page }) => {
  await admin(page, '/admin/stock')
  const before = await qty(page, '4790000000011')
  await page.getByRole('button', { name: 'Record write-off' }).click()
  await page.getByRole('button', { name: 'Product', exact: true }).click()
  await page.getByLabel('Find a product').fill('4790000000011')
  await page.getByLabel('Find a product').press('Enter')
  await page.getByRole('button', { name: 'Shop use' }).click()
  await page.getByLabel('Quantity').fill('2')
  await page.getByRole('button', { name: 'Write off', exact: true }).click()
  await toast(page, /shop use/)
  expect(await qty(page, '4790000000011')).toBe(before - 2)
})

test('labels: a product label opens as a printable sheet', async ({ page }) => {
  await admin(page, '/admin/labels')
  await page.getByRole('button', { name: /From a product/ }).click()
  await page.getByLabel('Find a product').fill('4790000000011')
  await page.getByLabel('Find a product').press('Enter')
  await page.getByRole('button', { name: '6', exact: true }).click()
  const [sheet] = await Promise.all([page.waitForEvent('popup'), page.getByRole('link', { name: /Sheet/ }).click()])
  await expect(sheet.getByLabel('Label 6')).toBeVisible()
  await expect(sheet.getByLabel('Label 7')).toHaveCount(0)
})

test.describe('phone (360px)', () => {
  test.use({ viewport: { width: 360, height: 780 } })

  test('products are cards, More reaches every page, stock actions work', async ({ page }) => {
    await admin(page, '/admin')
    await page.getByRole('button', { name: 'More' }).click()
    await page.getByRole('link', { name: 'Stock', exact: true }).click()
    await expect(page).toHaveURL(/\/admin\/stock$/)
    await page.getByLabel('Search products').fill('E2E Milk Crate')
    await expect(page.getByRole('list', { name: 'Stock levels' }).getByRole('button', { name: 'Adjust' })).toHaveCount(1) // filtered to the crate, with its action on the card
    const scroll = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
    expect(scroll).toBeLessThanOrEqual(0) // no sideways scroll
    await page.goto('/admin/products/review')
    await expect(page.getByRole('heading', { name: 'Items to review' })).toBeVisible()
  })
})
