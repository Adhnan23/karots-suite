import { describe, expect, it } from 'vitest'
import type { HeldSale } from '@/api/types.gen'
import { fromHeld, toHeldBlob } from './held'
import { initialState, tillReducer } from './store'

const held = (cart: string): HeldSale => ({
  id: 1, label: 'x', saleType: 'retail', discount: '10.00', cart, itemCount: 1, total: '90', createdAt: '',
})

describe('held sales', () => {
  it('round-trips lines, lots, serials and a percent bill discount', () => {
    let s = initialState('retail')
    s = tillReducer(s, { type: 'addService', line: { productId: 3, name: 'Job', price: 50 } })
    s = { ...s, billDiscountType: 'percent', lines: s.lines.map((l) => ({ ...l, batchId: 4, serials: ['A1'] })) }
    const back = fromHeld(held(toHeldBlob(s)), 10)
    expect(back.billDiscountType).toBe('percent')
    expect(back.lines[0]).toMatchObject({ productId: 3, batchId: 4, serials: ['A1'], key: 11 })
    expect(back.seq).toBe(11)
  })
  it('an unreadable cart resumes empty instead of crashing', () => {
    expect(fromHeld(held('{oops'), 0).lines).toEqual([])
  })
})
