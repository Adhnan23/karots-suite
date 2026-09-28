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
  it('resumes a karots-pos (array) held cart from a restored backup', () => {
    const legacy = JSON.stringify([{ id: 7, name: 'Soap', unit_price: 100, tax_rate: 0, qty: 2, stock: 10, is_service: false,
      allowDecimal: false, batch_id: 0, discount: '5', discountType: 'fixed', track_serial: false, warranty_months: 0,
      serials: [], price_override: '', description: '', components: [{ product_id: 9, quantity: '1' }] }])
    const back = fromHeld(held(legacy), 0)
    expect(back.lines[0]).toMatchObject({ productId: 7, unitPrice: 100, qty: 2, discount: '5', components: [{ productId: 9, quantity: '1' }] })
    expect(back.billDiscountType).toBe('fixed')
  })
  it('an unreadable cart resumes empty instead of crashing', () => {
    expect(fromHeld(held('{oops'), 0).lines).toEqual([])
  })
})
