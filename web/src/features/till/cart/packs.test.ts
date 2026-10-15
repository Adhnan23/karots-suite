import { describe, expect, it } from 'vitest'
import type { TillProduct } from '@/api/types.gen'
import fixture from './packs.fixture.json'
import { linePacks, packGross } from './packs'
import { initialState, tillReducer } from './store'
import { lineDiscount, lineGross, saleTotal } from './totals'

describe('packGross mirrors Go sales.PackGross', () => {
  for (const c of fixture) {
    it(c.case, () => {
      const p = { packs: c.packs.map((k, i) => ({ id: i, name: k.name, size: k.size, sellingPrice: k.selling, wholesalePrice: k.wholesale })) } as TillProduct
      const r = packGross(Number(c.qty), Number(c.unit), linePacks(p, c.wholesale ? 'wholesale' : 'retail'))
      expect(r.gross.toFixed(2)).toBe(c.gross)
      expect(r.label).toBe(c.label)
    })
  }
})

const panadol: TillProduct = { id: 7, name: 'Panadol', barcode: '', sellingPrice: '26.00', wholesalePrice: '0.00', taxRate: '0',
  stockQty: '100', unitAbbr: 'tab', unitAllowDecimal: false, isService: false, trackSerial: false, warrantyMonths: 0, lowStock: false, emoji: '', badges: [],
  packs: [{ id: 3, name: 'Card', size: '12', sellingPrice: '250.00', wholesalePrice: '0.00' }] }

describe('cart with packs', () => {
  it('adding a card adds its units to the same line; 14 rings 1 card + 2', () => {
    let s = tillReducer(initialState('retail'), { type: 'add', product: panadol, lots: [] })
    s = tillReducer(s, { type: 'add', product: panadol, lots: [], units: 12 })
    expect(s.lines).toHaveLength(1)
    expect(s.lines[0].qty).toBe(13)
    s = tillReducer(s, { type: 'setQty', key: s.lines[0].key, qty: 14, lots: [] })
    expect(lineGross(s.lines[0])).toBe(302)
    expect(saleTotal(s)).toBe(302)
  })
  it('a fixed discount is still per unit', () => {
    let s = tillReducer(initialState('retail'), { type: 'add', product: panadol, lots: [], units: 12 })
    s = tillReducer(s, { type: 'lineDiscount', key: s.lines[0].key, discount: '1', discountType: 'fixed' })
    expect(lineDiscount(s.lines[0])).toBe(12)
    expect(saleTotal(s)).toBe(238)
  })
  it('a pack set at the till joins the line and sets its qty', () => {
    const plain = { ...panadol, packs: [] }
    let s = tillReducer(initialState('retail'), { type: 'add', product: plain, lots: [] })
    expect(s.lines[0].packs).toBeUndefined()
    s = tillReducer(s, { type: 'linePacks', key: s.lines[0].key, packs: [{ id: 9, name: 'Card', size: 10, price: 0 }], qty: 20 })
    expect(s.lines[0].qty).toBe(20)
    expect(lineGross(s.lines[0])).toBe(520)
  })
})
