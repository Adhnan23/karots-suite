import { describe, expect, it } from 'vitest'
import type { StockLine } from '@/api/types.gen'
import { billTotals, lineNet, marginHint } from './totals'

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

describe('receive margin guard (karots-pos suggestSell / marginWarn)', () => {
  const cur = { cost: '50', sell: '75' } // 1.5× markup
  it('keeps the old markup on a dearer cost', () => {
    expect(marginHint(line({ costPrice: '60' }), cur)).toBe(90) // blank sell = 75 < 90
    expect(marginHint(line({ costPrice: '60', sellingPrice: '90' }), cur)).toBeNull()
  })
  it('flags selling at or below cost, and falls back to 20% with no prior cost', () => {
    expect(marginHint(line({ costPrice: '80', sellingPrice: '80' }), cur)).toBe(120)
    expect(marginHint(line({ costPrice: '100', sellingPrice: '110' }), { cost: '0', sell: '0' })).toBe(120)
    expect(marginHint(line({ costPrice: '40' }), cur)).toBeNull() // cheaper: fine
    expect(marginHint(line({ costPrice: '' }), cur)).toBeNull()
    expect(marginHint(line({ costPrice: '60' }))).toBeNull()
  })
})
