import { beforeEach, describe, expect, it } from 'vitest'
import type { ReorderRow } from '@/api/types.gen'
import { clearPrefill, linesFor, peekPrefill, savePrefill } from './prefill'

const row = (p: Partial<ReorderRow>): ReorderRow => ({
  productId: 1, name: 'Pencil', unitAbbr: 'pcs', onHand: '6', reorderLevel: 100, cost: '25.00', supplierId: 3, supplierName: 'Lanka', suggested: '164',
  fromSales: true, soldWeek: '', soldMonth: '', soldLastYear: '', note: '', covered: false, auto: true, looksOff: false, ...p,
})

describe('Low stock → Suppliers Order prefill', () => {
  beforeEach(() => sessionStorage.clear())
  it('turns rows into order lines at the suggested quantity and cost', () => {
    expect(linesFor([row({}), row({ productId: 2, name: 'Pen', suggested: '0', cost: '' })])).toEqual([
      { productId: 1, name: 'Pencil', ordered: '', quantity: '164', freeQty: '', costPrice: '25.00', sellingPrice: '', expiryDate: '', discount: '', discountType: 'fixed', packSize: '' },
    ])
  })
  it('is read only by the supplier it was made for, until cleared', () => {
    savePrefill(3, linesFor([row({})]))
    expect(peekPrefill(4)).toBeUndefined()
    expect(peekPrefill(3)?.[0].quantity).toBe('164')
    clearPrefill()
    expect(peekPrefill(3)).toBeUndefined()
  })
})
