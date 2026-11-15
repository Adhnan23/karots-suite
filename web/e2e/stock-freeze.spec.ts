import { expect, test, type Page } from './fixtures'

// Stock freeze (spec 2026-11-14): the pencil day through the screens. Needs `make e2e-seed`.
test.describe.configure({ mode: 'serial' })

async function login(page: Page, phone: string, to = '/till') {
  await page.context().clearCookies()
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}
async function ensureOpen(page: Page) {
  const dlg = page.getByRole('dialog', { name: 'Open the till' })
  if (await dlg.isVisible()) {
    await dlg.getByLabel(/Count of .*500\.00/).fill('2')
    await dlg.getByRole('button', { name: 'Open the till' }).click()
    await expect(dlg).toBeHidden()
    const skip = page.getByRole('button', { name: 'Skip' })
    await skip.waitFor({ state: 'visible', timeout: 1500 }).then(() => skip.click()).catch(() => undefined)
  }
}
const cart = (page: Page) => page.getByRole('region', { name: 'Cart' })
const stockOf = async (page: Page, id: number) => Number((await (await page.request.get(`/api/v1/till/products/${id}`)).json()).stockQty)

/** A pencil with 200 counted (admin session). */
async function pencil(page: Page, tag: string) {
  const cat = (await (await page.request.get('/api/v1/admin/categories')).json()).find((c: { name: string }) => c.name === 'E2E Goods')
  const code = `4799${Date.now() % 1_000_000_000}`
  const name = `E2E Pencil ${tag} ${Date.now() % 100000}`
  const res = await page.request.post('/api/v1/admin/products', { data: {
    name, nameLocal: '', barcode: code, categoryId: cat.id, unitId: 1, costPrice: '20', sellingPrice: '30', wholesalePrice: '', taxRate: '',
    reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } })
  expect(res.ok(), await res.text()).toBe(true)
  const id = (await res.json()).id as number
  expect((await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: id, newQuantity: '200', note: 'counted the back', batchId: 0 } })).ok()).toBe(true)
  return { id, code, name }
}

async function sellAll(page: Page) {
  await page.getByLabel('Amount tendered').press('Enter')
  await expect(page.getByRole('dialog', { name: 'Sell more than the count shows?' })).toHaveCount(0)
  await expect(page.getByRole('dialog', { name: /complete/ })).toBeVisible()
  await page.keyboard.press('Enter')
}
async function scanQty(page: Page, p: { code: string; name: string }, qty: string) {
  await page.locator('#till-search').fill(p.code)
  await page.keyboard.press('Enter')
  const q = cart(page).getByLabel(`Quantity of ${p.name}`)
  await expect(q).toHaveValue('1')
  await q.fill(qty)
  await q.press('Enter')
}

test('the pencil day: freeze, sell, open a box, shelf write-off, unfreeze', async ({ page }) => {
  await login(page, '0770000001', '/admin')
  const p = await pencil(page, 'day')
  await page.goto(`/admin/stock?q=${encodeURIComponent(p.name)}`)
  const row = page.getByRole('row', { name: new RegExp(p.name) }).or(page.getByRole('listitem').filter({ hasText: p.name })).first()
  await row.getByRole('button', { name: 'Freeze' }).click()
  await expect(row.getByText('frozen')).toBeVisible()

  await login(page, '0770000002')
  await ensureOpen(page)
  await scanQty(page, p, '4')
  await expect(cart(page).getByRole('button', { name: `Unfreeze ${p.name}` })).toBeVisible()
  await sellAll(page)
  expect(await stockOf(page, p.id)).toBe(200)

  await page.goto('/till/frozen')
  await page.getByRole('button', { name: new RegExp(p.name) }).click()
  await page.getByRole('button', { name: 'Moved to the shelf…' }).click()
  const shelf = page.getByRole('dialog', { name: 'Moved to the shelf' })
  await shelf.getByLabel('How many').fill('10')
  await shelf.getByRole('button', { name: 'Move to the shelf' }).click()
  await expect(shelf).toBeHidden()
  expect(await stockOf(page, p.id)).toBe(190)

  await page.goto('/till/write-off')
  await page.getByLabel('Find a product').fill(p.code)
  await page.getByLabel('Find a product').press('Enter')
  await page.getByRole('radio', { name: /From the shelf/ }).click()
  await page.getByLabel('Quantity').fill('1')
  await page.getByRole('button', { name: /from the shelf/ }).click()
  await expect(page.locator('[data-sonner-toast]').filter({ hasText: /Written off/ })).toBeVisible()
  expect(await stockOf(page, p.id)).toBe(190)

  await page.goto('/till/frozen')
  await page.getByRole('button', { name: new RegExp(p.name) }).click()
  await page.getByRole('button', { name: 'Unfreeze…' }).click()
  const thaw = page.getByRole('dialog', { name: `Unfreeze ${p.name}` })
  await thaw.getByLabel('On the shelf now').fill('13')
  await thaw.getByRole('button', { name: 'Unfreeze' }).click()
  await expect(thaw).toBeHidden()
  expect(await stockOf(page, p.id)).toBe(203)

  await page.goto('/till')
  await scanQty(page, p, '1')
  await sellAll(page)
  expect(await stockOf(page, p.id)).toBe(202)

  await login(page, '0770000001', '/admin')
  const dmg = await (await page.request.get(`/api/v1/admin/stock/movements?product=${p.id}&type=damage`)).json()
  expect(dmg.items).toHaveLength(1) // on the damaged list, with its cost
})

test('unfreeze in the middle of a sale: the cart stays', async ({ page }) => {
  await login(page, '0770000001', '/admin')
  const p = await pencil(page, 'mid')
  expect((await page.request.post('/api/v1/admin/stock/freeze', { data: { productIds: [p.id] } })).ok()).toBe(true)
  await login(page, '0770000002')
  await ensureOpen(page)
  await page.locator('#till-search').fill('4790000000011')
  await page.keyboard.press('Enter')
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
  await scanQty(page, p, '4')
  const url = page.url()
  await cart(page).getByRole('button', { name: `Unfreeze ${p.name}` }).click()
  const thaw = page.getByRole('dialog', { name: `Unfreeze ${p.name}` })
  await thaw.getByLabel('On the shelf now').fill('2')
  await thaw.getByRole('button', { name: 'Unfreeze' }).click()
  await expect(thaw).toBeHidden()
  expect(page.url()).toBe(url)
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
  await expect(cart(page).getByLabel(`Quantity of ${p.name}`)).toHaveValue('4')
  await expect(cart(page).getByRole('button', { name: `Unfreeze ${p.name}` })).toHaveCount(0)
  expect(await stockOf(page, p.id)).toBe(202)
  await sellAll(page)
  expect(await stockOf(page, p.id)).toBe(198)
})

test('a sales person unfreezes from the ticket screen', async ({ page }) => {
  await login(page, '0770000001', '/admin')
  const p = await pencil(page, 'seller')
  expect((await page.request.post('/api/v1/admin/stock/freeze', { data: { productIds: [p.id] } })).ok()).toBe(true)
  await login(page, '0770000004')
  await scanQty(page, p, '4')
  await cart(page).getByRole('button', { name: `Unfreeze ${p.name}` }).click()
  const thaw = page.getByRole('dialog', { name: `Unfreeze ${p.name}` })
  await thaw.getByLabel('On the shelf now').fill('2')
  await thaw.getByRole('button', { name: 'Unfreeze' }).click()
  await expect(thaw).toBeHidden()
  await expect(cart(page).getByLabel(`Quantity of ${p.name}`)).toHaveValue('4')
  expect(await stockOf(page, p.id)).toBe(202)
  await cart(page).getByLabel(`Quantity of ${p.name}`).fill('0') // leave no ticket behind
})

test('freeze the shop, then unfreeze it with the warning list', async ({ page }) => {
  await login(page, '0770000001', '/admin/stock?view=frozen')
  try {
    await page.getByRole('button', { name: 'Freeze the shop' }).click()
    await expect(page.getByText(/The shop is frozen/)).toBeVisible()
    const p = await pencil(page, 'new')
    expect((await (await page.request.get(`/api/v1/till/products/${p.id}`)).json()).stockFrozen).toBe(true)
    await page.reload()
    await page.getByRole('button', { name: 'Unfreeze the shop…' }).click()
    const warn = page.getByRole('dialog', { name: 'Unfreeze the whole shop?' })
    await expect(warn.getByRole('list', { name: 'Still frozen' }).getByText(p.name)).toBeVisible()
    await warn.getByRole('button', { name: 'Unfreeze all' }).click()
    await expect(page.getByText('Nothing is frozen')).toBeVisible()
  } finally {
    await page.request.post('/api/v1/admin/stock/unfreeze-shop')
  }
})
