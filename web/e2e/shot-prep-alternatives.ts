import type { Page } from './fixtures'

type Spec = { name: string; price: string; stock: string; off?: boolean }
type GroupSpec = { name: string; tiers: { name: string; level: number; items: Spec[] }[] }

// Alternatives' UI-gate data (the plugin must be built in): a tiered USB group with
// an out-of-stock item and a disabled one, a low cable group without tiers, and an
// untracked battery group. Once; returns the USB group's id.
export const ALT_GROUPS: GroupSpec[] = [
  { name: 'USB flash drive 32GB', tiers: [
    { name: 'Genuine', level: 10, items: [{ name: 'SanDisk Ultra 32GB', price: '1800', stock: '14' }, { name: 'Kingston DataTraveler 32GB', price: '1750', stock: '0' }, { name: 'Transcend 32GB (old)', price: '1700', stock: '6', off: true }] },
    { name: 'Normal', level: 5, items: [{ name: 'HP v150 32GB', price: '1400', stock: '3' }] },
    { name: 'Cheap', level: 0, items: [{ name: 'Generic USB 32GB', price: '900', stock: '25' }] },
  ] },
  { name: 'Phone cable Type-C', tiers: [{ name: '', level: 10, items: [{ name: 'Type-C cable 1m braided', price: '650', stock: '4' }, { name: 'Type-C cable 1m', price: '450', stock: '2' }] }] },
  { name: 'AA batteries (pair)', tiers: [{ name: '', level: 0, items: [{ name: 'Eveready AA pair', price: '220', stock: '40' }, { name: 'Panasonic AA pair', price: '260', stock: '18' }] }] },
]

export async function alternativesData(page: Page): Promise<number> {
  const base = '/api/v1/x/alternatives'
  const json = async (r: Promise<{ json(): Promise<unknown> }>) => (await r).json() as Promise<any>
  const have = (await json(page.request.get(`${base}?disabled=1`))) as { id: number; name: string }[]
  const found = have.find((g) => g.name === ALT_GROUPS[0].name)
  if (found) return found.id
  const cat = (await json(page.request.get('/api/v1/admin/categories')))[0]?.id
  const unit = (await json(page.request.get('/api/v1/admin/units')))[0]?.id
  let first = 0
  for (const gs of ALT_GROUPS) {
    let g = await json(page.request.post(base, { data: { name: gs.name } }))
    first ||= g.id
    for (const [i, t] of gs.tiers.entries()) {
      const tid = i === 0 ? g.tiers[0].id : (g = await json(page.request.post(`${base}/${g.id}/tiers`, { data: { name: t.name, reorderLevel: t.level } }))).tiers[i].id
      if (i === 0) await page.request.put(`${base}/tiers/${tid}`, { data: { name: t.name, reorderLevel: t.level } })
      for (const it of t.items) {
        const p = await json(page.request.post('/api/v1/admin/products', { data: { name: it.name, categoryId: cat, unitId: unit, costPrice: '0', sellingPrice: it.price } }))
        await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: p.id, newQuantity: it.stock, note: 'shot' } })
        await page.request.post(`${base}/tiers/${tid}/members`, { data: { productId: p.id } })
        if (it.off) await page.request.delete(`/api/v1/admin/products/${p.id}`)
      }
    }
  }
  return first
}
