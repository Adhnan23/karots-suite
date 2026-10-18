import { describe, expect, it } from 'vitest'
import { buildSaleRequest, checkoutGate } from './checkout'
import { initialState, tillReducer } from './store'
import { billDiscount, changeDue, goodsSubtotal, lineDiscount, payoutTotal, saleTotal } from './totals'
import type { TillState } from './types'

const cart = (): TillState => {
  let s = tillReducer(initialState('retail'), { type: 'addService', line: { productId: 1, name: 'Rice', price: 1000 } })
  s = tillReducer(s, { type: 'addPayout', line: { productId: 9, name: 'Cash out', amount: 5000, plugin: { x: { kind: 'cashout' } } } })
  return s
}
const ctx = { customer: null, canManageCredit: false, confirmed: false }

describe('payout lines (money handed to the customer)', () => {
  it('net against the goods: the cart totals below zero and the difference is change', () => {
    const s = cart()
    expect(payoutTotal(s.lines)).toBe(5000)
    expect(goodsSubtotal(s.lines)).toBe(1000)
    expect(saleTotal(s)).toBe(-4000)
    expect(changeDue(saleTotal(s), [])).toBe(4000)
  })
  it('are never discounted: a bill discount stays on the goods', () => {
    const s = { ...cart(), billDiscount: '10', billDiscountType: 'percent' as const }
    expect(lineDiscount(s.lines[1])).toBe(0)
    expect(billDiscount(s.lines, '10', 'percent')).toBe(100)
    expect(saleTotal(s)).toBe(-4100)
  })
  it('go out in cash only, and post as payout with the amount', () => {
    const s = cart()
    const g = checkoutGate(s, ctx)
    expect('ok' in g && g.payments).toEqual([])
    const req = buildSaleRequest(s, [], { deferDrawerKick: false })
    expect(req.items[1]).toMatchObject({ productId: 9, quantity: '1', priceOverride: '5000', payout: true, plugin: { x: { kind: 'cashout' } } })
    expect(req.items[0].payout).toBeUndefined()
    const card = { method: 'card', amount: '100', reference: '', accountId: '3', meta: {} }
    expect(checkoutGate({ ...s, tenders: [card] }, ctx)).toEqual({ error: expect.stringContaining('cash') })
  })
})
