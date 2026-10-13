import type { LotRow } from '@/api/types.gen'
import { dec, num } from '@/lib/money'

const byAge = (a: LotRow, b: LotRow) => a.createdAt.localeCompare(b.createdAt) || a.id - b.id

/** What the merge prompt starts from (mirrors Go stock.MergeDefaults): the
 * oldest batch is kept, cost = quantity-weighted average (2dp), the earliest
 * expiry, the oldest batch's own price ("" = follows the product). */
export function mergeDefaults(lots: LotRow[]) {
  const sorted = [...lots].sort(byAge)
  const qty = sorted.reduce((s, l) => s + num(l.qtyRemaining), 0)
  const value = sorted.reduce((s, l) => s + num(l.qtyRemaining) * num(l.costPrice), 0)
  const expiries = sorted.map((l) => l.expiryDate).filter(Boolean).sort()
  return {
    keptId: sorted[0]?.id ?? 0,
    qty,
    value,
    cost: qty > 0 ? dec(value / qty) : '0.00',
    expiry: expiries[0] ?? '',
    price: sorted[0]?.sellingPrice ?? '',
    expiriesDiffer: new Set(sorted.map((l) => l.expiryDate)).size > 1,
    pricesDiffer: new Set(sorted.map((l) => l.sellingPrice)).size > 1,
  }
}

/** The batch an intake restock should top up by default: same cost and the
 * same price it sells at — and the same expiry when one is typed. 0 = a new batch. */
export function matchBatch(lots: LotRow[], typed: { cost: string; selling: string; expiry: string }, productPrice: string) {
  const hit = [...lots].sort(byAge).find((l) =>
    num(l.costPrice) === num(typed.cost) &&
    num(l.sellingPrice || productPrice) === num(typed.selling || productPrice) &&
    (!typed.expiry || l.expiryDate === typed.expiry))
  return hit?.id ?? 0
}
