import { expect, test, type Page } from './fixtures'

// Product Plus (plugin, built in for Playwright). Needs `make e2e-seed`. Every
// field is scoped to this run's own category, so no other spec's product sees one.
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const CAT = `PP Cat ${tag}`
const OTHER = `PP Other ${tag}`
const GRADE = `Grade ${tag}` // required dropdown, made in the UI
const MODEL = `Model ${tag}` // searchable text: till ⓘ, label, intake
const COLOUR = `Colour ${tag}` // text with a default
const base = '/api/v1/x/productplus'
const ids = { cat: 0, other: 0, a: 0, b: 0 }

async function login(page: Page, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}

const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()
const json = async <T,>(p: Promise<{ json: () => Promise<unknown> }>) => (await (await p).json()) as T

async function product(page: Page, name: string, categoryId: number, barcode = '') {
  const res = await page.request.post('/api/v1/admin/products', { data: {
    name, nameLocal: '', barcode, categoryId, unitId: 1, costPrice: '40', sellingPrice: '60', wholesalePrice: '', taxRate: '',
    reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } })
  expect(res.ok()).toBe(true)
  return (await res.json()).id as number
}

async function field(page: Page, f: Record<string, unknown>) {
  const res = await page.request.post(`${base}/fields`, { data: {
    type: 'text', defaultValue: '', hint: '', required: false, searchable: false, showAtTill: false, printOnLabel: false,
    showInIntake: false, options: [], scope: [ids.cat], ...f,
  } })
  expect(res.ok()).toBe(true)
  return (await res.json()).id as number
}

const formValue = async (page: Page, productId: number, label: string) =>
  (await json<{ label: string; value: string }[]>(page.request.get(`${base}/form?productId=${productId}&categoryId=${ids.cat}&where=form`))).find((f) => f.label === label)?.value

async function pickCategory(page: Page, name: string) {
  await page.getByRole('dialog').getByRole('button', { name: 'Category' }).click()
  await page.getByLabel('Search categories').fill(name)
  await page.getByLabel('Search categories').press('Enter')
}

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage()
  await login(page, '/admin')
  const quick = (name: string) => json<{ id: number }>(page.request.post('/api/v1/admin/categories/quick', { data: { name, parentId: 0 } }))
  ids.cat = (await quick(CAT)).id
  ids.other = (await quick(OTHER)).id
  ids.a = await product(page, `PP Alpha ${tag}`, ids.cat, `PPA${tag}`)
  ids.b = await product(page, `PP Beta ${tag}`, ids.cat)
  await page.close()
})

test('a scoped required dropdown shows only in its category and blocks a blank save', async ({ page }) => {
  await login(page, '/admin/x/productplus')
  await page.getByRole('button', { name: 'Add field' }).click()
  await page.getByRole('textbox', { name: 'Label', exact: true }).fill(GRADE)
  await page.getByLabel('Type').selectOption('select')
  await page.getByLabel('Options').fill('A\nB\nC')
  await page.getByText('The product form won’t save without it.').click() // the Required toggle's label
  await page.getByRole('button', { name: 'Add a category' }).click()
  await page.getByLabel('Search categories').fill(CAT)
  await page.getByLabel('Search categories').press('Enter')
  await page.getByRole('button', { name: 'Add field', exact: true }).last().click()
  await toast(page, new RegExp(`${GRADE} added`))

  await login(page, '/admin/products')
  await page.getByRole('button', { name: 'New product' }).click()
  await page.getByLabel('Name', { exact: true }).fill(`PP New ${tag}`)
  await page.getByLabel('Selling').fill('99')
  await expect(page.getByRole('combobox', { name: GRADE })).toHaveCount(0)
  await pickCategory(page, OTHER)
  await expect(page.getByRole('combobox', { name: GRADE })).toHaveCount(0)
  await pickCategory(page, CAT)
  await expect(page.getByRole('combobox', { name: GRADE })).toBeVisible()
  await page.getByRole('button', { name: 'Add product' }).click()
  expect(await page.getByRole('combobox', { name: GRADE }).evaluate((el: HTMLSelectElement) => el.validity.valueMissing)).toBe(true)
  await expect(page.getByRole('dialog')).toBeVisible() // the browser held the blank required field
  await page.getByRole('combobox', { name: GRADE }).selectOption('B')
  await page.getByRole('button', { name: 'Add product' }).click()
  await toast(page, /added/)
  const id = (await json<{ items: { id: number }[] }>(page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(`PP New ${tag}`)}`))).items[0].id
  expect(await formValue(page, id, GRADE)).toBe('B')
})

test('a searchable value is found at the till and shows in ⓘ and the label lines', async ({ page }) => {
  await login(page, '/admin')
  await field(page, { label: MODEL, searchable: true, showAtTill: true, printOnLabel: true, showInIntake: true })
  const m = (await json<{ id: number; label: string }[]>(page.request.get(`${base}/fields`))).find((f) => f.label === MODEL)!
  await page.request.put(`${base}/values`, { data: [{ productId: ids.a, fieldId: m.id, value: `XJ-${tag}` }] })
  const found = await json<{ items: { id: number }[] }>(page.request.get(`/api/v1/till/products?search=xj-${tag}`))
  expect(found.items.map((p) => p.id)).toEqual([ids.a])
  const detail = await json<{ rows: { label: string; value: string }[] }>(page.request.get(`/api/v1/till/products/${ids.a}/detail`))
  expect(detail.rows).toContainEqual({ label: MODEL, value: `XJ-${tag}` })
  const label = await json<{ label: string; value: string }[]>(page.request.get(`/api/v1/till/products/${ids.a}/label-fields`))
  expect(label).toContainEqual({ label: MODEL, value: `XJ-${tag}` })
})

test('stock intake shows only the fields marked for it', async ({ page }) => {
  await login(page, '/admin/inventory/intake')
  await page.getByLabel('Scan or search').fill(`PP Intake ${tag}`)
  await page.getByLabel('Scan or search').press('Enter')
  await page.getByRole('button', { name: 'Category' }).click()
  await page.getByLabel('Search categories').fill(CAT)
  await page.getByLabel('Search categories').press('Enter')
  const more = page.getByRole('group', { name: 'More details' })
  await expect(more.getByRole('textbox', { name: MODEL })).toBeVisible()
  await expect(more.getByRole('combobox', { name: GRADE })).toHaveCount(0)
})

test('Fill in: Missing only, then save two cells', async ({ page }) => {
  await login(page, '/admin/x/productplus')
  await page.getByRole('row').filter({ hasText: GRADE }).getByRole('link', { name: /missing/ }).click()
  await expect(page.getByRole('button', { name: 'Missing only' })).toHaveAttribute('aria-pressed', 'true')
  await page.getByLabel('Search products').fill(tag)
  await page.getByRole('combobox', { name: `${GRADE} for PP Alpha ${tag}` }).selectOption('A')
  await page.getByRole('combobox', { name: `${GRADE} for PP Beta ${tag}` }).selectOption('C')
  await expect(page.getByText('2 changes')).toBeVisible()
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await toast(page, /2 saved/)
  await expect(page.getByText('Nothing missing')).toBeVisible()
  expect(await formValue(page, ids.b, GRADE)).toBe('C')
})

test('sheets: custom columns behind the toggle, and a value imports back', async ({ page }) => {
  await login(page, '/admin')
  const head = async (qs: string) => (await (await page.request.get(`/api/v1/admin/products/export?format=csv${qs}`)).text()).split('\n')[0]
  expect(await head('')).not.toContain(MODEL)
  expect(await head('&extras=productplus')).toContain(MODEL)

  const csv = await (await page.request.get('/api/v1/admin/products/export?format=csv&extras=productplus')).text()
  const lines = csv.split('\n')
  const row = lines.find((l) => l.includes(`PPA${tag}`))!
  const out = [lines[0], row.replace(`XJ-${tag}`, `XJ-${tag}-2`)].join('\n')
  const res = await page.request.post('/api/v1/admin/products/import?extras=productplus', { multipart: { file: { name: 'p.csv', mimeType: 'text/csv', buffer: Buffer.from(out) } } })
  expect(res.ok()).toBe(true)
  expect(await formValue(page, ids.a, MODEL)).toBe(`XJ-${tag}-2`)
})

test('a default change for new products only; a lossy type change asks first', async ({ page }) => {
  await login(page, '/admin')
  const colour = await field(page, { label: COLOUR, defaultValue: 'Red' })
  await page.request.put(`${base}/values`, { data: [{ productId: ids.b, fieldId: colour, value: 'Blue' }] })

  await page.goto('/admin/x/productplus')
  await page.getByRole('button', { name: new RegExp(`^${COLOUR}`) }).first().click()
  await page.getByRole('textbox', { name: 'Default' }).fill('Green')
  await expect(page.getByRole('radio', { name: /Only new products/ })).toBeChecked()
  await page.getByRole('button', { name: 'Save field' }).click()
  await toast(page, new RegExp(`${COLOUR} saved`))
  expect(await formValue(page, ids.a, COLOUR)).toBe('Red') // read the old default: kept
  expect(await formValue(page, ids.b, COLOUR)).toBe('Blue')
  expect(await formValue(page, 0, COLOUR)).toBe('Green') // a new product

  await page.getByRole('button', { name: new RegExp(`^${COLOUR}`) }).first().click()
  await page.getByLabel('Type').selectOption('number')
  await expect(page.getByRole('alert')).toContainText('will be deleted')
  await page.getByRole('button', { name: 'Save field' }).click()
  await toast(page, /Tick the box/)
  await page.keyboard.press('Escape')
  const f = (await json<{ label: string; type: string }[]>(page.request.get(`${base}/fields`))).find((x) => x.label === COLOUR)!
  expect(f.type).toBe('text')
})

test('Product Plus and Fill in fit a phone', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 })
  for (const path of ['/admin/x/productplus', '/admin/x/productplus/fill']) {
    await login(page, path)
    await expect(page.getByRole('heading', { name: 'Product Plus' })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  }
})
