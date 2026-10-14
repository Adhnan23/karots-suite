import { describe, expect, it } from 'vitest'
import type { TillProduct } from '@/api/types.gen'
import { initialState, tillReducer } from './store'

const p: TillProduct = { id: 1, name: 'Milk', barcode: '1', sellingPrice: '100.00', wholesalePrice: '0.00', taxRate: '0',
  stockQty: '5', unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial: false, warrantyMonths: 0, emoji: '', badges: [], packs: [] }

describe('tillReducer', () => {
  it('picking a customer un-picks the default cash tender; walk-in restores it', () => {
    let s = initialState('retail')
    expect(s.draft.method).toBe('cash')
    s = tillReducer(s, { type: 'customer', id: 5 })
    expect(s.draft.method).toBe('')
    s = tillReducer(s, { type: 'customer', id: null })
    expect(s.draft.method).toBe('cash')
  })
  it('never overrides a deliberate method or a typed amount', () => {
    let s = tillReducer(initialState('retail'), { type: 'draft', patch: { method: 'card' } })
    s = tillReducer(s, { type: 'customer', id: 5 })
    expect(s.draft.method).toBe('card')
  })
  it('commitDraft moves a priced draft into the tender list and keeps the method', () => {
    let s = tillReducer(initialState('retail'), { type: 'draft', patch: { amount: '500' } })
    s = tillReducer(s, { type: 'commitDraft' })
    expect(s.tenders).toHaveLength(1)
    expect(s.draft).toMatchObject({ method: 'cash', amount: '' })
  })
  it('quickSet sets (not adds) the quantity, idempotent on re-scan', () => {
    let s = tillReducer(initialState('retail'), { type: 'quickSet', product: p, qty: 3 })
    s = tillReducer(s, { type: 'quickSet', product: p, qty: 3 })
    expect(s.lines).toHaveLength(1)
    expect(s.lines[0].qty).toBe(3)
  })
  it('adding dismisses the post-sale change banner; newSale clears everything but sale type', () => {
    let s = tillReducer(initialState('wholesale'), { type: 'newSale', saleType: 'wholesale', change: 40 })
    expect(s.postSaleChange).toBe(40)
    s = tillReducer(s, { type: 'add', product: p, lots: [] })
    expect(s.postSaleChange).toBe(0)
    s = tillReducer(s, { type: 'newSale', saleType: 'wholesale' })
    expect(s.lines).toEqual([])
    expect(s.saleType).toBe('wholesale')
  })
  it('clears on demand without touching the (already empty) cart', () => {
    let s = tillReducer(initialState('retail'), { type: 'newSale', saleType: 'retail', change: 25 })
    s = tillReducer(s, { type: 'clearPostSaleChange' })
    expect(s.postSaleChange).toBe(0)
  })
})
