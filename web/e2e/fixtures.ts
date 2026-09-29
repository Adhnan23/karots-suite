import { test as base } from '@playwright/test'

export { expect, type Page } from '@playwright/test'

// Every spec signs in from one IP, so the suite outruns the login limiter
// (burst 10, +1 per 2s; unlock shares it). Wait out a 429 instead of failing.
export const test = base.extend({
  page: async ({ page }, provide) => {
    await page.route(/\/api\/v1\/auth\/(login|unlock)$/, async (route) => {
      try {
        for (let i = 0; ; i++) {
          const res = await route.fetch()
          if (res.status() !== 429 || i === 10) return await route.fulfill({ response: res })
          await new Promise((r) => setTimeout(r, 2100))
        }
      } catch {
        // the test ended mid-retry
      }
    })
    await provide(page)
  },
})

// Flows that need an expired lot make their own (a counter delivery dated three
// days ago), so specs never fight over the one lot the seed ships with.
// Needs a session with supplier access (admin / E2E Manager).
export async function receiveExpired(page: import('@playwright/test').Page, productId: number, qty = '2') {
  // Its own supplier, so these deliveries never move another spec's balances.
  let sup = (await (await page.request.get('/api/v1/till/suppliers?q=E2E%20Lot%20Source')).json())[0]
  if (!sup) sup = await (await page.request.post('/api/v1/till/suppliers', { data: { name: 'E2E Lot Source', phone: '' } })).json()
  const expiry = new Date(Date.now() - 3 * 86_400_000).toISOString().slice(0, 10)
  const res = await page.request.post(`/api/v1/till/suppliers/${sup.id}/receive`, { data: {
    invoiceNo: '', discount: '', discountType: 'fixed', notes: 'e2e expired lot', keepRemainder: false, deferDrawerKick: true,
    pay: { amount: '', method: 'cash', lockerId: 0 },
    lines: [{ productId, name: '', ordered: '', quantity: qty, freeQty: '', costPrice: '40', sellingPrice: '', expiryDate: expiry, discount: '', discountType: 'fixed' }],
  } })
  if (!res.ok()) throw new Error(`receive expired lot: ${res.status()} ${await res.text()}`)
}

/** A fresh product in E2E Goods (admin session), for flows that change its stock. */
export async function freshProduct(page: import('@playwright/test').Page, name: string): Promise<number> {
  const cat = (await (await page.request.get('/api/v1/admin/categories')).json()).find((c: { name: string }) => c.name === 'E2E Goods')
  const res = await page.request.post('/api/v1/admin/products', { data: {
    name, nameLocal: '', barcode: '', categoryId: cat.id, unitId: 1, costPrice: '40', sellingPrice: '60', wholesalePrice: '', taxRate: '',
    reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } })
  if (!res.ok()) throw new Error(`create product: ${res.status()} ${await res.text()}`)
  return (await res.json()).id
}
