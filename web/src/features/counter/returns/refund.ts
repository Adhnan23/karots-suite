import type { SaleLine } from '@/api/types.gen'
import { num, r2 } from '@/lib/money'

export const REASONS = ['Wrong item', 'Faulty', 'Changed mind', 'Expired']
/** How much of a line can still come back. */
export const left = (l: SaleLine) => num(l.quantity) - num(l.returnedQty)
/** What returning n of a line refunds — the server's sales.lineRefund: part of a line
 *  is its paid value pro-rata, the rest of it is exactly what's left of that value. */
export const refundOf = (l: SaleLine, n: number) => (n <= 0 ? 0 : n >= left(l) ? num(l.refundLeft) : r2((num(l.paid) * n) / num(l.quantity)))
