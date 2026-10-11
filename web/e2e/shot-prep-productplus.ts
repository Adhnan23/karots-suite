import type { Page } from './fixtures'

// Product Plus' UI-gate data and SHOTS_PREP steps (the plugin must be built in).
const PHONE = '4790000000066' // E2E Phone, in E2E Goods

// A required text field most products are missing, a dropdown with a default, a
// yes/no, and a date scoped to E2E Goods. Once.
export async function productPlusFields(page: Page) {
  const base = '/api/v1/x/productplus'
  const fields = await (await page.request.get(`${base}/fields?disabled=1`)).json()
  if (fields.some((f: { label: string }) => f.label === 'Model no')) return
  const cats: { id: number; name: string }[] = await (await page.request.get('/api/v1/admin/categories')).json()
  const goods = cats.find((c) => c.name === 'E2E Goods')
  const add = async (f: Record<string, unknown>) => (await (await page.request.post(`${base}/fields`, {
    data: { type: 'text', defaultValue: '', hint: '', required: false, searchable: false, showAtTill: false, printOnLabel: false, showInIntake: false, options: [], scope: [], ...f },
  })).json()).id as number
  const model = await add({ label: 'Model no', required: true, searchable: true, showAtTill: true, printOnLabel: true, showInIntake: true, hint: 'As printed on the box' })
  await add({ label: 'Grade', type: 'select', options: ['A', 'B', 'C'], defaultValue: 'A', showAtTill: true })
  await add({ label: 'Waterproof', type: 'bool', searchable: true, showInIntake: true })
  await add({ label: 'Best before', type: 'date', scope: goods ? [goods.id] : [] })
  const id = (await (await page.request.get(`/api/v1/till/products/barcode/${PHONE}`)).json()).id as number
  await page.request.put(`${base}/values`, { data: [{ productId: id, fieldId: model, value: 'SM-A055F' }] })
}

/** Opens a field's edit sheet by its label. */
export async function editField(page: Page, label: string) {
  await page.getByRole('button', { name: new RegExp(`^${label}\\b`) }).first().click()
}

/** Waits for the More details section and scrolls it into view. */
async function details(page: Page, block: ScrollLogicalPosition = 'end') {
  const g = page.getByRole('group', { name: 'More details' })
  await g.waitFor()
  await g.evaluate((el, b) => el.scrollIntoView({ block: b }), block)
}

async function fresh(page: Page) {
  await productPlusFields(page)
  await page.reload()
  await page.waitForLoadState('networkidle')
}

/** SHOTS_PATH=/admin/products: E2E Phone's edit sheet. */
async function editPhone(page: Page) {
  await fresh(page)
  await page.getByLabel('Search products').fill(PHONE)
  await page.getByText('E2E Phone', { exact: true }).locator('visible=true').first().click()
  await details(page)
}

export const ppPrep: Record<string, (page: Page) => Promise<void>> = {
  /** Product Plus fields list (SHOTS_PATH=/admin/x/productplus). */
  'pp-fields': fresh,
  /** Edit sheet: Model no switched to Number (lossy warning). */
  'pp-type': async (page) => {
    await fresh(page)
    await editField(page, 'Model no')
    await page.getByLabel('Type').selectOption('number')
    await page.getByRole('alert').waitFor()
  },
  /** Edit sheet: Grade's default changed (only new / all choice). */
  'pp-default': async (page) => {
    await fresh(page)
    await editField(page, 'Grade')
    await page.getByRole('combobox', { name: 'Default' }).selectOption('B')
  },
  /** Product form, editing: own value, a default, the scoped date. */
  'pp-form-edit': editPhone,
  'pp-form-dup': async (page) => {
    await editPhone(page)
    await page.getByRole('button', { name: 'Duplicate' }).click()
    await page.getByText('Duplicate product').waitFor()
    await details(page)
  },
  /** New product, no category yet: only the all-products fields. */
  'pp-form-new': async (page) => {
    await fresh(page)
    await page.getByRole('button', { name: 'New product' }).click()
    await details(page)
  },
  /** E2E Phone moved to another category: Best before drops out. */
  'pp-form-out': async (page) => {
    await page.request.post('/api/v1/admin/categories/quick', { data: { name: 'Shot Clothing', parentId: 0 } })
    await editPhone(page)
    await page.getByRole('dialog').getByRole('button', { name: 'Category' }).click()
    await page.getByLabel('Search categories').fill('Shot Clothing')
    await page.getByLabel('Search categories').press('Enter')
    await page.waitForLoadState('networkidle')
    await details(page)
  },
  /** SHOTS_PATH=/admin/inventory/intake: New item with the intake fields. */
  'pp-intake': async (page) => {
    await fresh(page)
    await page.getByLabel('Scan or search').fill('Rain jacket')
    await page.getByLabel('Scan or search').press('Enter')
    await details(page, 'center') // the page scrolls: clear the phone tabs
  },
  /** SHOTS_PATH=/admin/inventory/intake: restock E2E Phone, label top line = Model no (the Label chip). */
  'pp-label': async (page) => {
    await fresh(page)
    await page.getByLabel('Scan or search').fill(PHONE)
    await page.getByLabel('Scan or search').press('Enter')
    await page.getByLabel('Top line').selectOption({ label: 'Model no' })
    await page.getByLabel('Top line').evaluate((el) => el.scrollIntoView({ block: 'center' }))
  },
}
