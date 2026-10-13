import type { Page } from './fixtures'

/** SHOTS_PREP=product-barcodes, SHOTS_PATH=/admin/products: E2E Milk's form scrolled to its
 * Other barcodes (adds two extras when it has none). */
export const barcodesPrep: Record<string, (page: Page) => Promise<void>> = {
  'product-barcodes': async (page) => {
    const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
    const have = await (await page.request.get(`/api/v1/admin/products/${milk.id}/barcodes`)).json()
    if (have.length < 2) for (const barcode of ['8901030865278', '6001234500017'])
      await page.request.post(`/api/v1/admin/products/${milk.id}/barcodes`, { data: { barcode, source: 'form' } })
    await page.getByLabel('Search products').fill('E2E Milk')
    await page.getByRole('button', { name: /^E2E Milk\b/ }).first().click()
    await page.getByRole('region', { name: 'Other barcodes' }).scrollIntoViewIfNeeded()
  },
}

/** SHOTS_PREP=intake-unknown (SHOTS_PATH=/admin/inventory/intake) and purchase-unknown
 * (SHOTS_PATH=/admin/purchases/new): an unknown code scanned, its new-item panel open. */
Object.assign(barcodesPrep, {
  'intake-unknown': async (page: Page) => {
    await page.getByLabel('Scan or search').fill('6009876543213')
    await page.getByLabel('Scan or search').press('Enter')
    await page.getByRole('button', { name: 'Same as an existing one' }).waitFor()
  },
  'purchase-unknown': async (page: Page) => {
    const input = page.getByLabel('Add a product')
    await input.fill('6009876543213')
    await input.press('Enter')
    await page.getByRole('button', { name: 'Same as an existing one' }).waitFor()
  },
})

/** SHOTS_PREP=merge-review (SHOTS_PATH=/admin/products/review): a quick-added "Milk 1L" (Rs 90,
 * 4 on hand at its own price) → Same as an existing one → E2E Milk: the merge confirm with a
 * price choice. */
Object.assign(barcodesPrep, {
  'merge-review': async (page: Page) => {
    const have = await (await page.request.get('/api/v1/admin/products/review')).json()
    if (!have.some((p: { name: string }) => p.name === 'Milk 1L')) {
      const p = await (await page.request.post('/api/v1/till/quick-item', { data: { name: 'Milk 1L', price: '90', qty: '1', barcode: '6001111222233', unitId: 0 } })).json()
      const id = p.id ?? p.productId
      if (id) await page.request.post('/api/v1/admin/inventory/intake/restock', { data: { productId: id, quantity: '4', costPrice: '', sellingPrice: '', wholesalePrice: '', scanned: '' } })
    }
    await page.reload()
    await page.waitForLoadState('networkidle')
    await page.getByRole('button', { name: 'Same as an existing one' }).first().click()
    await page.getByLabel('Find a product').fill('E2E Milk')
    await page.getByRole('button', { name: /^E2E Milk\b/ }).first().click()
    await page.getByRole('button', { name: 'Merge', exact: true }).waitFor()
  },
})
