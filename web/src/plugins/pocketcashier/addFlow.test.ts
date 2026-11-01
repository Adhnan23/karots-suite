import { describe, expect, it } from 'vitest'
import type { CopyProduct, Lot, TillCopy } from '@/api/types.gen'
import { addFlow } from './addFlow'

const product = (over: Partial<CopyProduct> = {}) => ({
  id: 7, name: 'Soap', sellingPrice: '100.00', wholesalePrice: '0.00', isService: false, packs: [], ...over,
}) as unknown as CopyProduct
const lot = (batchId: number, price: string, expiryDate = ''): Lot =>
  ({ batchId, price, qtyRemaining: '5', expiryDate, batchNo: '', receivedAt: '2026-10-01' }) as Lot
const copy = (lots: Lot[] = []) => ({ priceOptions: lots.length ? { 7: lots } : {} }) as unknown as TillCopy
const box = { id: 3, name: 'Box', size: '12', sellingPrice: '1100.00', wholesalePrice: '0.00' }

describe('adding on the phone', () => {
  it('asks which price when lots disagree', () => {
    const lots = [lot(1, '100.00'), lot(2, '120.00')]
    expect(addFlow(copy(lots), product())).toEqual({ kind: 'price', lots })
  })

  it('adds straight away with one lot', () => {
    expect(addFlow(copy([lot(1, '100.00')]), product())).toEqual({ kind: 'add', lots: [] })
  })

  it('asks loose or by the pack for a product sold by the pack', () => {
    expect(addFlow(copy(), product({ packs: [box] as CopyProduct['packs'] }))).toEqual({ kind: 'pack', lots: [] })
  })

  it("adds the pack when its own code was scanned", () => {
    expect(addFlow(copy(), product({ packs: [box] as CopyProduct['packs'] }), 3)).toEqual({ kind: 'add', lots: [], units: 12 })
  })

  it('never asks about packs on a service', () => {
    expect(addFlow(copy(), product({ isService: true, packs: [box] as CopyProduct['packs'] }))).toEqual({ kind: 'add', lots: [] })
  })
})
