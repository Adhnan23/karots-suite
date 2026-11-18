import type { SaleLine, SaleView } from '@/api/types.gen'
import { num, r2 } from '@/lib/money'

export const REASONS = ['Wrong item', 'Faulty', 'Changed mind', 'Expired']
/** How much of a line can still come back (a payout — money handed over — never does). */
export const left = (l: SaleLine) => (num(l.subtotal) < 0 ? 0 : num(l.quantity) - num(l.returnedQty))
/** What returning n of a line refunds — the server's sales.lineRefund: part of a line
 *  is its paid value pro-rata, the rest of it is exactly what's left of that value. */
export const refundOf = (l: SaleLine, n: number) =>
  n <= 0 ? 0 : n >= left(l) ? num(l.refundLeft) : partRefund(num(l.paid), num(l.quantity), num(l.returnedQty), n)

/** The server's sales.PartRefund: the value of everything back after this return less
 *  that of what was back before, each paid × part ÷ qty rounded half up — so the
 *  roundings cancel and a line never refunds past what was paid (proved in
 *  proofs/Karots/Refund.lean). Whole numbers (cents, thousandths of a unit): float
 *  maths put a .5 cent either side (TILL-2). */
function partRefund(paid: number, qty: number, before: number, n: number) {
  const P = Math.round(paid * 100), Q = Math.round(qty * 1000), R = Math.round(before * 1000), N = Math.round(n * 1000)
  const worth = (m: number) => Math.floor((2 * P * m + Q) / (2 * Q))
  return Q > 0 ? (worth(R + N) - worth(R)) / 100 : 0
}
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

/** How a refund of value splits, as the server pays it: what's still owed on the sale
 *  comes off the customer's account first (RET-18); then, when the cashier gives the
 *  sale's uncleared cheque back ('back'), up to the cheque's amount goes back as the
 *  cheque (RET-17); the rest is cash. pendingCheque > 0 = the screen must ask. */
export function refundSplit(sale: SaleView, value: number, way: 'back' | 'cash' | '') {
  const pendingCheque = r2(sale.payments.filter((p) => p.method === 'cheque' && p.chequeStatus === 'pending').reduce((s, p) => s + num(p.amount), 0))
  const offAccount = Math.min(value, num(sale.owedOnSale))
  const chequeBack = way === 'back' ? r2(Math.min(value - offAccount, pendingCheque)) : 0
  return { offAccount, chequeBack, cash: r2(value - offAccount - chequeBack), pendingCheque }
}
