import type { SaleView, TillCopy } from '@/api/types.gen'
import { refundOf } from '@/features/counter/returns/refund'
import { dec, num } from '@/lib/money'
import type { Action } from './store/outbox'
import type { CallInput } from './sync'

const CREDIT = /\/till\/customers\/(\d+)\/(payment|refund)$/
const WRITE_OFF = /\/till\/write-off$/
const CONVERT = /\/till\/conversions\/(\d+)\/run$/

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
  const stock = new Map<number, number>()
  const add = (id: number, n: number) => stock.set(id, (stock.get(id) ?? 0) + n)
  for (const c of callsIn(out)) {
    const body = c.body as { productId?: number; quantity?: string; toQty?: string }
    if (WRITE_OFF.test(c.path)) add(body.productId ?? 0, -num(body.quantity))
    const m = CONVERT.exec(c.path)
    const pack = m && copy.conversions?.find((x) => x.id === Number(m[1]))
    if (pack) {
      add(pack.fromId, -num(body.quantity))
      add(pack.toId, body.toQty ? num(body.toQty) : num(body.quantity) * num(pack.ratio))
    }
  }
  if (!moved.size && !stock.size) return copy
  const moves = (id: number, was: string) => (stock.has(id) ? String(r3(num(was) + stock.get(id)!)) : was)
  return {
    ...copy,
    customers: copy.customers.map((c) => (moved.has(c.id) ? { ...c, balance: dec(num(c.balance) + moved.get(c.id)!) } : c)),
    products: stock.size ? copy.products.map((p) => ({ ...p, stockQty: moves(p.id, p.stockQty) })) : copy.products,
    conversions: stock.size ? copy.conversions?.map((r) => ({ ...r, fromStock: moves(r.fromId, r.fromStock), toStock: moves(r.toId, r.toStock) })) : copy.conversions,
  }
}

const r3 = (n: number) => Math.round(n * 1000) / 1000

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
