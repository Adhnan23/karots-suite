import type { SaleView, TillCopy } from '@/api/types.gen'
import { refundOf } from '@/features/counter/returns/refund'
import { dec, num } from '@/lib/money'
import type { Action } from './store/outbox'
import type { CallInput } from './sync'

const CREDIT = /\/till\/customers\/(\d+)\/(payment|refund)$/

const callsIn = (out: Action[]) => out.filter((a) => a.kind === 'call').map((a) => a.body as CallInput)

/** The copy as the phone should show it: what the PC has told it, plus what this
 * phone did that the PC hasn't heard yet. */
export function withOverlay(copy: TillCopy, out: Action[]): TillCopy {
  const moved = new Map<number, number>()
  for (const c of callsIn(out)) {
    const m = CREDIT.exec(c.path)
    if (!m) continue
    const amount = num((c.body as { amount?: string }).amount)
    moved.set(Number(m[1]), (moved.get(Number(m[1])) ?? 0) + (m[2] === 'payment' ? -amount : amount))
  }
  if (!moved.size) return copy
  return { ...copy, customers: copy.customers.map((c) => (moved.has(c.id) ? { ...c, balance: dec(num(c.balance) + moved.get(c.id)!) } : c)) }
}

/** A sale with this phone's waiting returns taken off, valued as the server will. */
export function saleWithOverlay(sale: SaleView, out: Action[]): SaleView {
  const waiting = callsIn(out).filter((c) => c.path.endsWith(`/till/sales/${sale.id}/return`))
  if (!waiting.length) return sale
  let lines = sale.lines
  let owed = num(sale.owedOnSale)
  for (const c of waiting) {
    const back = (c.body as { lines?: { saleItemId: number; quantity: string }[] }).lines ?? []
    lines = lines.map((l) => {
      const b = back.find((x) => x.saleItemId === l.id)
      if (!b) return l
      const value = refundOf(l, num(b.quantity))
      owed = Math.max(0, owed - value)
      return { ...l, returnedQty: String(num(l.returnedQty) + num(b.quantity)), refundLeft: dec(num(l.refundLeft) - value) }
    })
  }
  return { ...sale, lines, owedOnSale: dec(owed) }
}
