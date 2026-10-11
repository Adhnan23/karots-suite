import type { Page } from './fixtures'

// Product Plus' UI-gate data (the plugin must be built in): a required text field
// most products are missing, a dropdown with a default, a yes/no, and a date
// scoped to one category. Once.
export async function productPlusFields(page: Page) {
  const base = '/api/v1/x/productplus'
  const fields = await (await page.request.get(`${base}/fields?disabled=1`)).json()
  if (fields.some((f: { label: string }) => f.label === 'Model no')) return
  const cats = await (await page.request.get('/api/v1/admin/categories')).json()
  const add = async (f: Record<string, unknown>) => (await (await page.request.post(`${base}/fields`, {
    data: { type: 'text', defaultValue: '', hint: '', required: false, searchable: false, showAtTill: false, printOnLabel: false, showInIntake: false, options: [], scope: [], ...f },
  })).json()).id as number
  const model = await add({ label: 'Model no', required: true, searchable: true, showAtTill: true, printOnLabel: true, hint: 'As printed on the box' })
  await add({ label: 'Grade', type: 'select', options: ['A', 'B', 'C'], defaultValue: 'A', showAtTill: true })
  await add({ label: 'Waterproof', type: 'bool', searchable: true, showInIntake: true })
  await add({ label: 'Best before', type: 'date', scope: cats[0] ? [cats[0].id] : [] })
  const byCode = async (c: string) => (await (await page.request.get(`/api/v1/till/products/barcode/${c}`)).json()).id as number
  await page.request.put(`${base}/values`, { data: [{ productId: await byCode('4790000000066'), fieldId: model, value: 'SM-A055F' }] })
}

/** Opens a field's edit sheet by its label. */
export async function editField(page: Page, label: string) {
  await page.getByRole('button', { name: new RegExp(`^${label}\\b`) }).first().click()
}
