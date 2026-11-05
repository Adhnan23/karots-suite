import { describe, expect, it } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import { expectedCash } from './drawer'
import { withOverlay } from './overlay'
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
})
