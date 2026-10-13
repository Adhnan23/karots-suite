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
