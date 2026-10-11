import type { Page } from './fixtures'

// Customer requests' UI-gate data (the plugin must be built in): an open request
// asked three times, one waiting for stock, one to call, one won't stock. Once.
export async function requestsWithHistory(page: Page) {
  const has = await (await page.request.get('/api/v1/x/requests/similar?q=shot')).json()
  if (has.length) return
  const post = async (url: string, data: unknown = {}) => {
    const res = await page.request.post(`/api/v1/x/requests${url}`, { data })
    return res.status() === 204 ? {} : res.json()
  }
  const add = async (note: string, name = '', phone = '') => (await post('', { note, name, phone })).id as number
  const byCode = async (c: string) => (await (await page.request.get(`/api/v1/till/products/barcode/${c}`)).json()).id as number
  const umbrella = await add('Shot: umbrella, red, large', 'Nimal Perera', '0771234567')
  await post(`/${umbrella}/asks`, { name: 'Kamala', phone: '0712223334' })
  await post(`/${umbrella}/asks`, { name: '', phone: '' })
  await add('Shot: 55-inch smart TV wall bracket')
  const call = await add('Shot: full-cream milk 1L', 'Sunil', '0779998887')
  await post(`/${call}/link`, { productId: await byCode('4790000000011') })
  const cat = (await (await page.request.get('/api/v1/admin/categories')).json())[0]?.id
  const unit = (await (await page.request.get('/api/v1/admin/units')).json())[0]?.id
  const wait = await add('Shot: rechargeable torch', 'Mala', '0715556667')
  await post(`/${wait}/product`, { name: 'Rechargeable torch (shot)', categoryId: cat, unitId: unit, sellingPrice: '2450', costPrice: '', supplierId: 0 })
  await post(`/${await add('Shot: umbrella with torch')}/decline`, { reason: 'supplier stopped making them' })
}
