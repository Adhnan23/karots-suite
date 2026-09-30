import { describe, expect, it } from 'vitest'
import type { StockLine } from '@/api/types.gen'
import { billTotals, lineNet } from './totals'

const line = (p: Partial<StockLine>): StockLine => ({
  productId: 1, name: 'x', ordered: '', quantity: '10', freeQty: '', costPrice: '60', sellingPrice: '', expiryDate: '', discount: '', discountType: 'fixed', ...p,
})

describe('supplier delivery totals (match purchases on the server)', () => {
  it('fixed line discount is per unit, percent is off the line', () => {
    expect(lineNet(line({ discount: '5' }))).toBe(550)
    expect(lineNet(line({ discount: '10', discountType: 'percent' }))).toBe(540)
    expect(lineNet(line({ discount: '999' }))).toBe(0) // never below zero
  })
  it('bill discount comes off the net after line discounts', () => {
    const ls = [line({ discount: '10', discountType: 'percent' })]
    expect(billTotals(ls, '20', 'fixed')).toEqual({ net: 540, bill: 20, total: 520 })
    expect(billTotals(ls, '10', 'percent').total).toBe(486)
  })
})
