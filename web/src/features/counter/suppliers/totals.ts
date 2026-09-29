import type { StockLine } from '@/api/types.gen'
import { num, r2 } from '@/lib/money'

// Mirrors purchases.resolveItemDiscount / resolveBillDiscount so the screen shows
// exactly what the server books: fixed = Rs per unit (line) or flat (bill);
// percent = off qty × cost (line) or off the net after line discounts (bill).
const clamp = (amt: number, base: number) => Math.min(Math.max(r2(amt), 0), Math.max(base, 0))

export const lineGross = (l: StockLine) => num(l.quantity) * num(l.costPrice)
export const lineDiscount = (l: StockLine) =>
  clamp(l.discountType === 'percent' ? (lineGross(l) * num(l.discount)) / 100 : num(l.discount) * num(l.quantity), lineGross(l))
export const lineNet = (l: StockLine) => r2(lineGross(l) - lineDiscount(l))

export function billTotals(lines: StockLine[], discount: string, type: string) {
  const net = r2(lines.reduce((s, l) => s + lineNet(l), 0))
  const bill = clamp(type === 'percent' ? (net * num(discount)) / 100 : num(discount), net)
  return { net, bill, total: r2(net - bill) }
}
