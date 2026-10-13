import { describe, expect, it } from 'vitest'
import type { LotRow } from '@/api/types.gen'
import { matchBatch, mergeDefaults } from './batches'

const lot = (id: number, qty: string, cost: string, extra: Partial<LotRow> = {}): LotRow => ({
  id, batchNo: '', expiryDate: '', qtyReceived: qty, qtyRemaining: qty, costPrice: cost, sellingPrice: '', source: 'adjust',
  createdAt: `2026-10-0${id}T10:00:00Z`, fromDelivery: false, ...extra,
})

describe('mergeDefaults', () => {
  it('keeps the oldest, weights the cost, takes the earliest expiry and the oldest price', () => {
    const d = mergeDefaults([
      lot(2, '10', '60.00', { expiryDate: '2027-05-01', sellingPrice: '120.00' }),
      lot(1, '4', '50.00', { sellingPrice: '100.00' }),
      lot(3, '1', '60.00', { expiryDate: '2027-03-12' }),
    ])
    expect(d).toMatchObject({ keptId: 1, qty: 15, cost: '57.33', expiry: '2027-03-12', price: '100.00', expiriesDiffer: true, pricesDiffer: true })
  })
  it('says nothing differs when the batches agree', () => {
    const d = mergeDefaults([lot(1, '2', '10.00'), lot(2, '3', '10.00')])
    expect(d).toMatchObject({ cost: '10.00', expiry: '', price: '', expiriesDiffer: false, pricesDiffer: false })
  })
})

describe('matchBatch', () => {
  const lots = [lot(1, '4', '50.00'), lot(2, '6', '60.00', { sellingPrice: '130.00', expiryDate: '2027-01-31' })]
  it('matches cost and the price the batch sells at', () => {
    expect(matchBatch(lots, { cost: '50', selling: '120', expiry: '' }, '120.00')).toBe(1)
    expect(matchBatch(lots, { cost: '60', selling: '130', expiry: '' }, '120.00')).toBe(2)
  })
  it('a typed expiry must match too', () => {
    expect(matchBatch(lots, { cost: '60', selling: '130', expiry: '2027-01-31' }, '120.00')).toBe(2)
    expect(matchBatch(lots, { cost: '60', selling: '130', expiry: '2027-02-28' }, '120.00')).toBe(0)
  })
  it('a new price or cost is a new batch', () => {
    expect(matchBatch(lots, { cost: '50', selling: '125', expiry: '' }, '120.00')).toBe(0)
    expect(matchBatch(lots, { cost: '55', selling: '120', expiry: '' }, '120.00')).toBe(0)
  })
})
