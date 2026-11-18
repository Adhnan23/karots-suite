import type { SaleLine } from '@/api/types.gen'
import { num, r2 } from '@/lib/money'

export const REASONS = ['Wrong item', 'Faulty', 'Changed mind', 'Expired']
/** How much of a line can still come back (a payout — money handed over — never does). */
export const left = (l: SaleLine) => (num(l.subtotal) < 0 ? 0 : num(l.quantity) - num(l.returnedQty))
/** What returning n of a line refunds — the server's sales.lineRefund: part of a line
 *  is its paid value pro-rata, the rest of it is exactly what's left of that value. */
export const refundOf = (l: SaleLine, n: number) => (n <= 0 ? 0 : n >= left(l) ? num(l.refundLeft) : r2((num(l.paid) * n) / num(l.quantity)))
/** Bumps how much of a line is coming back by d. A combo's lines move together, one
 *  whole set at a time, and stop at the sets still out — a group comes back only
 *  whole (RET-13; the server refuses anything else). */
export function bumpQty(lines: SaleLine[], qty: Record<number, number>, l: SaleLine, d: number): Record<number, number> {
  const sets = num(l.groupSets)
  if (!l.group || sets <= 0) return { ...qty, [l.id]: Math.min(left(l), Math.max(0, (qty[l.id] ?? 0) + d)) }
  const group = lines.filter((g) => g.group === l.group)
  const perSet = (g: SaleLine) => num(g.quantity) / sets
  const out = Math.min(...group.map((g) => Math.floor(left(g) / perSet(g) + 1e-9)))
  const now = Math.round((qty[l.id] ?? 0) / perSet(l))
  const k = Math.min(out, Math.max(0, now + d))
  const next = { ...qty }
  for (const g of group) next[g.id] = r2(k * perSet(g))
  return next
}
