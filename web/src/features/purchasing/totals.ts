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

export type Current = { cost: string; sell: string }

/** Units in what the line counts: a pack size when bought by the pack, else 1. */
export const lineSize = (l: StockLine) => (num(l.packSize) > 1 ? num(l.packSize) : 1)

/** The receive margin guard. The suggestion keeps the product's current
 * markup on the new cost (20% when it has none); a line is flagged when it would
 * sell at or below cost, or under that suggestion. Blank sell = the current price. */
export function marginHint(l: StockLine, cur?: Current): number | null {
  const cost = num(l.costPrice) / lineSize(l) // per unit, like the selling price
  if (!cur || !(cost > 0)) return null
  const curCost = num(cur.cost)
  const curSell = num(cur.sell)
  const sell = num(l.sellingPrice) || curSell
  const suggest = r2(curCost > 0 && curSell > 0 ? (cost * curSell) / curCost : cost * 1.2)
  return (sell > 0 && cost >= sell) || suggest > sell + 0.005 ? suggest : null
}
