import { describe, expect, it } from 'vitest'
import type { SaleView, TillCopy } from '@/api/types.gen'
import { expectedCash } from './drawer'
import { saleWithOverlay, withOverlay } from './overlay'
import type { Action } from './store/outbox'

const copy = { customers: [{ id: 1, name: 'Sunil', phone: '', balance: '900.00', creditLimit: '0' }], sessions: [{ userId: 7, expected: '1000.00' }] } as unknown as TillCopy
const call = (path: string, body: unknown, extra: object = {}): Action => ({ ref: 'r', kind: 'call', userId: 7, at: '', body: { method: 'POST', path, body, label: '', ...extra } })

describe('what waits on the phone shows in the copy', () => {
  it('a waiting payment lowers the balance shown', () => {
    const c = withOverlay(copy, [call('/api/v1/till/customers/1/payment', { amount: '300' })])
    expect(c.customers[0].balance).toBe('600.00')
  })
  it('a waiting advance refund raises it', () => {
    expect(withOverlay(copy, [call('/api/v1/till/customers/1/refund', { amount: '50' })]).customers[0].balance).toBe('950.00')
  })
  it('leaves the copy alone with nothing waiting', () => {
    expect(withOverlay(copy, [])).toBe(copy)
  })
  it('cash a call takes or gives moves the drawer', () => {
    const out = [call('/x', {}, { cashIn: '300' }), call('/y', {}, { cashOut: '50' })]
    expect(expectedCash(copy, 7, out)).toBe('1250.00')
  })
  it('a waiting return lowers what is left of the line and the debt on the sale', () => {
    const sale = { id: 5, owedOnSale: '100.00', lines: [{ id: 11, quantity: '3', returnedQty: '0', subtotal: '300.00', paid: '300.00', refundLeft: '300.00' }] } as unknown as SaleView
    const s = saleWithOverlay(sale, [call('/api/v1/till/sales/5/return', { lines: [{ saleItemId: 11, quantity: '2' }] })])
    expect(s.lines[0].returnedQty).toBe('2')
    expect(s.lines[0].refundLeft).toBe('100.00')
    expect(s.owedOnSale).toBe('0.00')
  })
  it('a return for another sale changes nothing', () => {
    const sale = { id: 6, owedOnSale: '0.00', lines: [] } as unknown as SaleView
    expect(saleWithOverlay(sale, [call('/api/v1/till/sales/5/return', { lines: [] })])).toBe(sale)
  })
})
