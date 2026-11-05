import { dec, num } from '@/lib/money'
import type { TillService } from '@/plugins/documents/types'
import type { OfflineRoute, RouteCtx } from '../fetch'

type Size = string | null
type PriceRow = { size: Size; material: string; color: boolean; double: boolean; minQty: number; unitPrice: string }
type PaperRow = { size: Size; material: string; productId: number; qtyPerUnit: string }
type RecipeRow = { productId: number; qtyPerUnit: string | null; yieldUnits: string | null; wholeUnits: boolean }
export type DocPart = { services: TillService[]; prices: Record<string, PriceRow[]>; paper: Record<string, PaperRow[]>; recipes: Record<string, RecipeRow[]> }
type Job = { size: string; material: string; color: boolean; double: boolean; copies: number }

// The PC's rules, from plugins/documents/resolve.go. documents.fixture.json is read
// by tests on both sides, so a change on one side fails until the other follows.
function pickPrice(rows: PriceRow[], j: Job): PriceRow | null {
  let best: PriceRow | null = null
  for (const r of rows) {
    if (r.material !== j.material || r.color !== j.color || r.double !== j.double || r.minQty > j.copies) continue
    if (r.size !== null && r.size !== j.size) continue
    const own = r.size === j.size
    const bestOwn = best?.size === j.size
    if (!best || (own !== bestOwn ? own : r.minQty > best.minQty)) best = r
  }
  return best
}

function paperFor(all: PaperRow[], j: Job): PaperRow[] {
  const mine = all.filter((p) => p.material === j.material)
  const own = mine.filter((p) => p.size === j.size)
  return own.length ? own : mine.filter((p) => p.size === null)
}

/** Whole numbers of a millionth, so 0.0166665 rounds as the PC's decimals do. */
function consumed(r: RecipeRow, copies: number): number {
  const yields = r.yieldUnits ? num(r.yieldUnits) : 0
  const per8 = yields > 0 ? Math.round(1e8 / yields) : r.qtyPerUnit !== null ? Math.round(num(r.qtyPerUnit) * 1e8) : 0
  const used = (per8 * copies) / 100 // millionths
  return r.wholeUnits ? Math.ceil(used / 1e6) : Math.round(used) / 1e6
}

export function offlineQuote(part: DocPart, serviceId: number, j: Job) {
  const service = part.services.find((s) => s.id === serviceId)
  const price = pickPrice(part.prices[serviceId] ?? [], j)
  if (!service || !price || j.copies <= 0) return null
  const sheets = j.double ? Math.floor((j.copies + 1) / 2) : j.copies
  const components: { productId: number; quantity: string }[] = []
  const seen = new Set<number>()
  for (const p of paperFor(part.paper[serviceId] ?? [], j)) {
    const q = Math.ceil(num(p.qtyPerUnit) * sheets)
    if (q <= 0) continue
    seen.add(p.productId)
    components.push({ productId: p.productId, quantity: String(q) })
  }
  for (const r of part.recipes[service.productId] ?? []) {
    if (seen.has(r.productId)) continue
    const q = consumed(r, j.copies)
    if (q > 0) components.push({ productId: r.productId, quantity: String(q) })
  }
  return { unitPrice: price.unitPrice, total: dec(num(price.unitPrice) * j.copies), minQty: price.minQty, sheets, components, service }
}

function description(s: TillService, j: Job): string {
  const parts = [j.size ? `${s.name} ${j.size}` : s.name]
  if (j.material) parts.push(j.material)
  if (s.category !== 'laminate' && s.category !== 'bind') parts.push(j.color ? 'Colour' : 'B&W', j.double ? '2-side' : '1-side')
  return parts.join(' · ')
}

function quote(c: RouteCtx) {
  const part = c.parts.documents as DocPart | undefined
  const q = c.query
  const job: Job = { size: q.get('size') ?? '', material: q.get('material') ?? '', color: q.get('color') === 'true', double: q.get('double') === 'true', copies: Number(q.get('copies')) || 0 }
  const got = part && offlineQuote(part, Number(q.get('service')), job)
  if (!got) return null
  const { service, ...rest } = got
  return { ...rest, description: description(service, job), cost: '0.00' } // no costs on the phone
}

export const documentsRoutes: OfflineRoute[] = [
  { method: 'GET', path: '/x/documents/till', answer: (c) => (c.parts.documents as DocPart | undefined)?.services ?? [] },
  { method: 'GET', path: '/x/documents/quote', answer: quote },
  { method: 'POST', path: '/x/documents/jobs', queue: true, label: () => 'Print & Copy job record', answer: () => ({}) },
  { method: 'POST', path: '/x/documents/own-use', queue: true, label: () => 'Print & Copy: shop use', answer: () => ({}) },
]
