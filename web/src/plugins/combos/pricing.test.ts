import { describe, expect, it } from 'vitest'
import { setLines, setPrice } from './pricing'
import type { Combo, Member } from './types'

const m = (productId: number, price: string, qty = '1', wholesale = '0.00'): Member =>
  ({ productId, name: `P${productId}`, qty, sellingPrice: price, wholesalePrice: wholesale, active: true, isService: false, stock: '9' })
const combo = (discountType: Combo['discountType'], discountValue: string, members: Member[]) => ({ discountType, discountValue, members })

/** What the lines charge for one set, in cents. */
const charged = (c: ReturnType<typeof combo>, saleType: 'retail' | 'wholesale' = 'retail') => {
  const price = new Map(c.members.map((x) => [x.productId, saleType === 'wholesale' && Number(x.wholesalePrice) > 0 ? x.wholesalePrice : x.sellingPrice]))
  return setLines(c, saleType).reduce((s, l) => s + Math.round((Number(price.get(l.productId)) - Number(l.discount)) * 100) * l.per, 0)
}

describe('combo pricing', () => {
  it('combo price: Milk 380 + Cookies 220 at Rs 560 is exactly 560 with the cent on one line', () => {
    const c = combo('price', '560', [m(1, '380.00'), m(2, '220.00')])
    expect(setLines(c, 'retail')).toEqual([{ productId: 1, per: 1, discount: '25.34' }, { productId: 2, per: 1, discount: '14.66' }])
    expect(charged(c)).toBe(56000)
  })
  it('Rs off on one product ×4 (buy 3 get 1)', () => {
    const c = combo('fixed', '95', [m(1, '95.00', '4')])
    expect(setLines(c, 'retail')).toEqual([{ productId: 1, per: 4, discount: '23.75' }])
    expect(charged(c)).toBe(28500)
  })
  it('splits a line when only some units need the leftover cent', () => {
    const c = combo('fixed', '10', [m(1, '33.00', '3')]) // 1000 / 3 units = 333 r 1
    const lines = setLines(c, 'retail')
    expect(lines).toEqual([{ productId: 1, per: 2, discount: '3.33' }, { productId: 1, per: 1, discount: '3.34' }])
    expect(charged(c)).toBe(9900 - 1000)
  })
  it('percent rounds the set once, then spreads exactly', () => {
    const c = combo('percent', '15', [m(1, '199.99'), m(2, '49.95', '3')])
    const { full, off } = setPrice(c, 'retail')
    expect(charged(c)).toBe(full - off)
  })
  it('wholesale works from the wholesale price (not retail)', () => {
    const c = combo('fixed', '50', [m(1, '300.00', '1', '250.00'), m(2, '200.00', '1', '150.00')])
    expect(setPrice(c, 'wholesale')).toEqual({ full: 40000, off: 5000, price: 35000 })
    expect(charged(c, 'wholesale')).toBe(35000)
  })
  it('never discounts past the full price; 100% off is free', () => {
    expect(charged(combo('fixed', '9999', [m(1, '10.00'), m(2, '5.00')]))).toBe(0)
    expect(charged(combo('percent', '100', [m(1, '10.00'), m(2, '5.00')]))).toBe(0)
    expect(charged(combo('price', '99999', [m(1, '10.00'), m(2, '5.00')]))).toBe(1500)
  })
})

// CMB-3 over random sets: the lines charge exactly the set's price, no line is
// discounted below 0 or past its own price, and every unit of every member is rung.
describe('combo pricing, random sets (CMB-3)', () => {
  let seed = 7
  const rnd = (n: number) => { seed = (seed * 1103515245 + 12345) % 2 ** 31; return seed % n }
  it('adds up exactly for 3,000 sets', () => {
    for (let i = 0; i < 3000; i++) {
      const members = Array.from({ length: 1 + rnd(4) }, (_, k) => m(k + 1, (1 + rnd(50000) / 100).toFixed(2), String(1 + rnd(5)), rnd(2) ? (1 + rnd(40000) / 100).toFixed(2) : '0.00'))
      const type = (['percent', 'fixed', 'price'] as const)[rnd(3)]
      const value = type === 'percent' ? String(rnd(101)) : (rnd(200000) / 100 + (type === 'price' ? 0.01 : 0)).toFixed(2)
      const c = combo(type, value, members)
      for (const saleType of ['retail', 'wholesale'] as const) {
        const { full, off, price } = setPrice(c, saleType)
        expect(off).toBeGreaterThanOrEqual(0)
        expect(off).toBeLessThanOrEqual(full)
        expect(charged(c, saleType)).toBe(price)
        const lines = setLines(c, saleType)
        for (const mem of members) expect(lines.filter((l) => l.productId === mem.productId).reduce((s, l) => s + l.per, 0)).toBe(Number(mem.qty))
        for (const l of lines) {
          const mem = members.find((x) => x.productId === l.productId)!
          const unit = saleType === 'wholesale' && Number(mem.wholesalePrice) > 0 ? Number(mem.wholesalePrice) : Number(mem.sellingPrice)
          expect(Number(l.discount)).toBeGreaterThanOrEqual(0)
          expect(Math.round(Number(l.discount) * 100)).toBeLessThanOrEqual(Math.round(unit * 100))
        }
      }
    }
  })
})
