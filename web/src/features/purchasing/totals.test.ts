import { describe, expect, it } from 'vitest'
import type { StockLine } from '@/api/types.gen'
import { billTotals, lineNet, marginHint } from './totals'

const line = (p: Partial<StockLine>): StockLine => ({
  productId: 1, name: 'x', ordered: '', quantity: '10', freeQty: '', costPrice: '60', sellingPrice: '', expiryDate: '', discount: '', discountType: 'fixed', packSize: '', ...p,
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

describe('bought by the pack', () => {
  it('owes qty × pack cost and judges margin per unit', () => {
    const l = line({ quantity: '3', costPrice: '250', packSize: '12', sellingPrice: '26' })
    expect(lineNet(l)).toBe(750)
    expect(marginHint(l, { cost: '20.83', sell: '26' })).toBeNull() // 20.83 a tablet, as before
    expect(marginHint(l, { cost: '20', sell: '26' })).toBe(27.08) // dearer now: keep the markup
  })
  it('switching to a card keeps the cost the same per unit', async () => {
    const { packSwitch } = await import('./LineEditor')
    expect(packSwitch(line({ costPrice: '20.84' }), 12)).toEqual({ packSize: '12', costPrice: '250.08' })
    expect(packSwitch(line({ costPrice: '250', packSize: '12' }), 1)).toEqual({ packSize: '', costPrice: '20.83' })
  })
})
