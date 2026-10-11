import type { SaleType } from '@/features/till/cart/types'
import type { GroupLineInput } from '@/plugins/slots'
import type { Combo, DiscountType, Member } from './types'

const cents = (s: string) => Math.round(Number(s) * 100)
const str = (c: number) => (c / 100).toFixed(2)

/** The price a member rings at: wholesale on a wholesale sale when it has one (the till's unitPriceFor rule). */
export const unitCents = (m: Member, saleType: SaleType) =>
  saleType === 'wholesale' && cents(m.wholesalePrice) > 0 ? cents(m.wholesalePrice) : cents(m.sellingPrice)

/** One set's full price and its discount, in cents. The discount never passes the full price. */
export function setOff(type: DiscountType, value: string, full: number): number {
  const v = Number(value) || 0
  const off = type === 'percent' ? Math.round((full * v) / 100) : type === 'fixed' ? cents(value) : full - cents(value)
  return Math.min(full, Math.max(0, off))
}

export function setPrice(c: Pick<Combo, 'discountType' | 'discountValue' | 'members'>, saleType: SaleType) {
  const full = c.members.reduce((s, m) => s + unitCents(m, saleType) * Number(m.qty), 0)
  const off = setOff(c.discountType, c.discountValue, full)
  return { full, off, price: full - off }
}

/** One set's lines with fixed per-unit discounts that add up to exactly the set's
 * discount: each unit gets floor(price × off / full) and the leftover cents go +1
 * onto units, splitting a line in two when only some of its units need it. */
export function setLines(c: Pick<Combo, 'discountType' | 'discountValue' | 'members'>, saleType: SaleType): GroupLineInput[] {
  const { full, off } = setPrice(c, saleType)
  const base = c.members.map((m) => {
    const unit = unitCents(m, saleType)
    return { productId: m.productId, per: Number(m.qty), unit, d: full > 0 ? Math.floor((unit * off) / full) : 0 }
  })
  let left = off - base.reduce((s, l) => s + l.d * l.per, 0)
  const out: GroupLineInput[] = []
  for (const l of base) {
    // Whole units only take the +1 (a member's qty in a set is a count of units).
    const k = l.d + 1 <= l.unit ? Math.min(l.per, left) : 0
    left -= k
    if (l.per - k > 0) out.push({ productId: l.productId, per: l.per - k, discount: str(l.d) })
    if (k > 0) out.push({ productId: l.productId, per: k, discount: str(l.d + 1) })
  }
  return out
}
