import type { TillProduct } from '@/api/types.gen'
import type { CartLine, ServiceLineInput } from '@/features/till/cart/types'
import { recordJobs } from './api'
import type { Extra, LineExtra, LineJob, Quote, TillOffer, TillService } from './types'

export const BADGE = '🖨' // plugins/documents/store.go Badge

/** The till's Print & Copy menu claims a picked product by its 🖨 badge. */
export const claims = (p: TillProduct) => p.badges.includes(BADGE)

export type Pick = { size: string; material: string; color: boolean; double: boolean }

/** Laminate and bind are priced by size and type only: no ink, no sides (plugins/documents Plain). */
export const plain = (s: { category: string }) => s.category === 'laminate' || s.category === 'bind'

/** Whether the service offers a choice on this axis at all (else the panel hides the row). */
export const varies = (s: TillService, axis: 'color' | 'double') => new Set(s.offers.map((o) => o[axis])).size > 1

/** Sizes on offer, in grid order ('' = any size, only when no named size exists). */
export function sizes(s: TillService): string[] {
  const all = [...new Set(s.offers.map((o) => o.size))]
  const named = all.filter(Boolean)
  return named.length ? named : all
}

/** Offers for a size: its own cells, plus any-size cells it doesn't override. */
export function offersFor(s: TillService, size: string): TillOffer[] {
  const own = s.offers.filter((o) => o.size === size && size !== '')
  const any = s.offers.filter((o) => o.size === '' && !own.some((x) => x.material === o.material && x.color === o.color && x.double === o.double))
  return [...own, ...any]
}

/** Types on offer for a size, in grid order ('' = no type). */
export const materials = (s: TillService, size: string) => [...new Set(offersFor(s, size).map((o) => o.material))]

export const offerOf = (s: TillService, p: Pick) =>
  offersFor(s, p.size).find((o) => o.material === p.material && o.color === p.color && o.double === p.double)

/** Where the panel starts: the size with the cheapest copy (A4 over A3, as a rule). */
export function startPick(s: TillService): Pick {
  const base = (size: string) => Math.min(...offersFor(s, size).map((o) => Number(o.bands[0]?.price ?? Infinity)))
  const size = [...sizes(s)].sort((a, b) => base(a) - base(b))[0] ?? ''
  return settle(s, { size, material: '', color: false, double: false })
}

/** A pick that exists: keep what can be kept, else the first offer of the size. */
export function settle(s: TillService, p: Pick): Pick {
  const size = sizes(s).includes(p.size) ? p.size : (sizes(s)[0] ?? '')
  const ms = materials(s, size)
  const material = ms.includes(p.material) ? p.material : (ms[0] ?? '')
  const os = offersFor(s, size).filter((o) => o.material === material)
  const hit = os.find((o) => o.color === p.color && o.double === p.double)
    ?? os.find((o) => o.color === p.color) ?? os.find((o) => o.double === p.double) ?? os[0]
  return hit ? { size, material, color: hit.color, double: hit.double } : { size, material, color: false, double: false }
}

/** The cart line of a priced metered job: qty fixed (its band and paper depend on it). */
export function meteredLine(s: TillService, p: Pick, copies: number, q: Quote): ServiceLineInput {
  const job: LineJob = plain(s)
    ? { serviceId: s.id, size: p.size, material: p.material, sheets: q.sheets }
    : { serviceId: s.id, size: p.size, material: p.material, color: p.color, double: p.double, sheets: q.sheets }
  return { productId: s.productId, name: q.description, description: q.description, price: Number(q.total), qty: copies,
    unitPrice: Number(q.unitPrice), components: q.components, plugin: { documents: job }, fixedQty: true }
}

/** A custom job (CV, photo edit): the typed price, the note on the receipt line. */
export function customLine(s: TillService, price: number, note: string): ServiceLineInput {
  const description = note.trim() ? `${s.name} — ${note.trim()}` : s.name
  return { productId: s.productId, name: description, description, price, qty: 1, unitPrice: price, plugin: { documents: { serviceId: s.id } } }
}

/** How many of an extra a job takes: one per job, or one per piece. */
const extraCount = (x: Extra, pieces: number) => (x.perPiece ? Math.max(1, pieces) : 1)

/** What the ticked extras add to a job of this many pieces. */
export const extrasTotal = (xs: Extra[], pieces: number) => xs.reduce((t, x) => t + Number(x.price) * extraCount(x, pieces), 0)

/** An extra's own cart line beside its job: "Laminate + Urgent", its stock taken with it. */
export function extraLine(s: TillService, x: Extra, pieces: number): ServiceLineInput {
  const n = extraCount(x, pieces)
  const description = `${s.name} + ${x.name}`
  const plugin: { documents: LineExtra } = { documents: { extraOf: s.id } }
  return { productId: s.productId, name: description, description, price: Number(x.price) * n, qty: n, unitPrice: Number(x.price),
    components: x.productId ? [{ productId: x.productId, quantity: String(Number(x.qty) * n) }] : [], plugin, fixedQty: x.perPiece }
}

const jobOf = (l: CartLine) => l.plugin?.documents as LineJob | undefined

/** After a sale: one ledger job per Print & Copy line. */
export function soldJobs(lines: CartLine[]) {
  return lines.flatMap((l) => {
    const j = jobOf(l)
    if (!j?.serviceId) return []
    return [{ serviceId: j.serviceId, description: l.description || l.name, size: j.size ?? null, material: j.material ?? '',
      color: j.color ?? null, double: j.double ?? null, sheets: j.sheets ?? null }]
  })
}

export async function afterSale({ saleId, lines }: { saleId: number; lines: CartLine[] }) {
  const jobs = soldJobs(lines)
  if (jobs.length) await recordJobs(saleId, jobs)
}
