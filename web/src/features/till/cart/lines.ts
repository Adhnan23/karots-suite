import type { Lot, TillProduct } from '@/api/types.gen'
import { num, qtyText } from '@/lib/money'
import { linePacks } from './packs'
import type { CartLine, Notice, SaleType, ServiceLineInput } from './types'

export const MAX_STOCK = Number.MAX_SAFE_INTEGER
export type LinesResult = { lines: CartLine[]; seq: number; key: number; notice: Notice }

const localToday = () => new Date().toLocaleDateString('en-CA') // YYYY-MM-DD, local

/** A lot is expired the day AFTER its printed date; no expiry never expires. */
export const lotExpired = (lot: Lot, today = localToday()) => lot.expiryDate !== '' && lot.expiryDate < today

/** Lots worth asking about: prices disagree, or the lot FEFO sells first is expired. */
export function lotsFor(options: Record<string, Lot[]> | undefined, productId: number, today = localToday()): Lot[] {
  const lots = options?.[String(productId)] ?? []
  if (!lots.length) return []
  if (new Set(lots.map((l) => l.price)).size > 1) return lots
  if (lotExpired(lots[0], today)) return lots
  return []
}

/** The few words that match a lot to the package in the cashier's hand. */
export function lotLabel(lot: Lot) {
  if (lot.expiryDate) return `exp ${lot.expiryDate}`
  if (lot.batchNo) return `batch ${lot.batchNo}`
  return `in ${lot.receivedAt}`
}

export const unitPriceFor = (p: TillProduct, saleType: SaleType) =>
  saleType === 'wholesale' && num(p.wholesalePrice) > 0 ? num(p.wholesalePrice) : num(p.sellingPrice)

/** One serial box per unit, keeping what was typed. */
export function syncSerials(l: CartLine): CartLine {
  if (!l.trackSerial) return l
  const n = Math.max(0, Math.round(l.qty))
  const serials = l.serials.slice(0, n)
  while (serials.length < n) serials.push('')
  return { ...l, serials }
}

export function newLine(key: number, p: TillProduct, saleType: SaleType, lot?: Lot): CartLine {
  return syncSerials({
    key, productId: p.id, name: p.name,
    unitPrice: lot ? num(lot.price) : unitPriceFor(p, saleType),
    taxRate: num(p.taxRate), qty: 1,
    stock: p.isService ? MAX_STOCK : num(p.stockQty),
    isService: p.isService, allowDecimal: p.unitAllowDecimal,
    batchId: lot?.batchId ?? 0, batchRemaining: lot ? num(lot.qtyRemaining) : 0, batchLabel: lot ? lotLabel(lot) : '',
    allowOversell: false, discount: '', discountType: 'fixed',
    trackSerial: p.trackSerial, warrantyMonths: p.warrantyMonths, serials: [],
    priceOverride: '', description: '', components: [],
    ...(p.packs?.length && !p.isService && { packs: linePacks(p, saleType) }),
  })
}

/** Add one unit (or a pack's worth). A line is product AND lot — merging on product alone would re-price. */
export function addProduct(lines: CartLine[], seq: number, p: TillProduct, saleType: SaleType, lot: Lot | undefined, lots: Lot[], units = 1, plugin?: Record<string, unknown>): LinesResult {
  const batchId = lot?.batchId ?? 0
  const tag = JSON.stringify(plugin ?? null) // a plugin's line (e.g. for a prescription) stays its own line
  const hit = lines.find((l) => l.productId === p.id && l.batchId === batchId && !l.isService && !l.group && JSON.stringify(l.plugin ?? null) === tag)
  if (hit) return setQty(lines, seq, hit.key, hit.qty + units, lots)
  const key = seq + 1
  const added = [...lines, { ...newLine(key, p, saleType, lot), ...(plugin && { plugin }) }]
  return units === 1 ? { lines: added, seq: key, key, notice: null } : setQty(added, key, key, units, lots)
}

/** Put another product on a plain line (an in-stock alternative), keeping its qty
 * and place — or taking qty when given (another unit: 1 bag ≠ 1 kg, so the till
 * asks); discount, lot, serials and found-at-till start over. A plain line of
 * that product already in the cart takes the qty instead. */
export function swapProduct(lines: CartLine[], key: number, p: TillProduct, saleType: SaleType, qty?: number): CartLine[] {
  const l = lines.find((x) => x.key === key)
  if (!l || l.group || l.isService) return lines
  const want = qty && qty > 0 ? qty : l.qty
  const hit = lines.find((x) => x.key !== key && x.productId === p.id && x.batchId === 0 && !x.group && !x.isService)
  if (hit) return lines.filter((x) => x.key !== key).map((x) => (x === hit ? syncSerials({ ...x, qty: x.qty + want }) : x))
  const q = p.unitAllowDecimal ? want : Math.max(1, Math.round(want))
  return lines.map((x) => (x.key === key ? syncSerials({ ...newLine(key, p, saleType), qty: q }) : x))
}

export function serviceLine(key: number, d: ServiceLineInput): CartLine {
  const qty = d.qty && d.qty > 0 ? d.qty : 1
  const unit = d.unitPrice ?? d.price
  return {
    key, productId: d.productId, name: d.name, unitPrice: unit, taxRate: 0, qty, stock: MAX_STOCK,
    isService: true, allowDecimal: false, batchId: 0, batchRemaining: 0, batchLabel: '', allowOversell: false,
    discount: '', discountType: 'fixed', trackSerial: false, warrantyMonths: 0, serials: [],
    priceOverride: String(unit), description: d.description ?? '', components: d.components ?? [], plugin: d.plugin,
    ...(d.fixedQty && { fixedQty: true }),
  }
}

/** Set a line's qty: clamp ≥ 0, whole units round, serials follow, lots spill. */
export function setQty(lines: CartLine[], seq: number, key: number, qty: number, lots: Lot[]): LinesResult {
  const next = lines.map((l) => {
    if (l.key !== key || l.fixedQty) return l
    let q = Math.max(0, num(qty))
    if (!l.allowDecimal) q = Math.round(q)
    return syncSerials({ ...l, qty: q })
  })
  return spill(next, seq, key, lots)
}

/** A picked lot holding fewer than asked: clamp to it and open a line on the next
 * lot at ITS price — two receipt lines, matching what leaves the shelf. */
function spill(lines: CartLine[], seq: number, key: number, lots: Lot[]): LinesResult {
  const l = lines.find((x) => x.key === key)
  if (!l || !l.batchId || !(l.batchRemaining > 0) || l.qty <= l.batchRemaining) return { lines, seq, key, notice: null }
  const over = l.qty - l.batchRemaining
  const clamped = syncSerials({ ...l, qty: l.batchRemaining })
  let next = lines.map((x) => (x.key === key ? clamped : x))
  const lot = lots.find((o) => o.batchId !== l.batchId && num(o.qtyRemaining) > 0 && !next.some((x) => x.productId === l.productId && x.batchId === o.batchId))
  if (!lot) {
    return { lines: next, seq, key, notice: { tone: 'error', text: `Only ${qtyText(l.batchRemaining)} left at ${l.unitPrice.toFixed(2)}` } }
  }
  const k2 = seq + 1
  next = [...next, syncSerials({ ...l, key: k2, unitPrice: num(lot.price), qty: over, batchId: lot.batchId,
    batchRemaining: num(lot.qtyRemaining), batchLabel: lotLabel(lot), serials: [] })]
  const deeper = spill(next, k2, k2, lots)
  return {
    lines: deeper.lines, seq: deeper.seq, key,
    notice: deeper.notice ?? { tone: 'success', text: `Split: ${qtyText(clamped.qty)} at ${l.unitPrice.toFixed(2)}, ${qtyText(over)} at ${num(lot.price).toFixed(2)}` },
  }
}
