import { describe, expect, it } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import type { Action } from '../../store/outbox'
import type { RouteCtx } from '../fetch'
import { coreRoutes } from './core'

const copy = { symbol: 'Rs.', askToPrint: true, defaultSaleType: 'retail', tenderMethods: [], priceOptions: { 4: [{ batchId: 1 }] }, denominations: [{ value: '100', isNote: true }],
  people: [{ id: 7, role: 'cashier', canManageCredit: true, canSeeCost: false }], places: [{ id: 2, name: 'Safe', kind: 'safe', cashierAccess: true }, { id: 3, name: 'Hidden', kind: 'safe', cashierAccess: false }],
  sessions: [{ id: 9, userId: 7, openedAt: 'x', openingCash: '100', expected: '500.00' }], customers: [{ id: 1, name: 'Sunil', phone: '077', balance: '900.00', creditLimit: '0' }],
  products: [{ id: 4, name: 'Panadol', barcode: '', barcodes: [], packCodes: [], isService: false, sellingPrice: '10', badges: [], categoryId: 1 }] } as unknown as TillCopy
const ctx = (over: Partial<RouteCtx> = {}): RouteCtx => ({ parts: {}, copy, params: {}, query: new URLSearchParams(), body: undefined, ref: 'r1', out: [], userId: 7, ...over })
const route = (m: string, p: string) => coreRoutes.find((r) => r.method === m && r.path === p)!
const payment: Action = { ref: 'p', kind: 'call', userId: 7, at: '', body: { method: 'POST', path: '/api/v1/till/customers/1/payment', label: '', body: { amount: '300' }, cashIn: '300' } }

describe('the till, answered from the copy', () => {
  it('config says what the person may do', () => {
    expect(route('GET', '/till/config').answer(ctx())).toMatchObject({ symbol: 'Rs.', askToPrint: true, canManageCredit: true, sendsToCashier: false, openCashDrawer: false })
  })
  it('the drawer counts what waits, and only places a cashier may use', () => {
    const d = route('GET', '/till/drawer').answer(ctx({ out: [payment] })) as { summary: { expected: string; session?: { id: number } }; lockers: { id: number }[] }
    expect(d.summary.expected).toBe('800.00')
    expect(d.summary.session?.id).toBe(9)
    expect(d.lockers.map((l) => l.id)).toEqual([2])
  })
  it('a closed drawer has no session', () => {
    const d = route('GET', '/till/drawer').answer(ctx({ userId: 8 })) as { summary: { session?: unknown } }
    expect(d.summary.session).toBeUndefined()
  })
  it('customers show waiting payments', () => {
    expect(route('GET', '/till/customers').answer(ctx({ out: [payment] }))).toEqual([{ id: 1, name: 'Sunil', phone: '077', creditLimit: '0', outstanding: '600.00' }])
  })
  it('products search and read one', () => {
    expect(route('GET', '/till/products').answer(ctx({ query: new URLSearchParams('search=pan') }))).toMatchObject({ total: 1 })
    expect(route('GET', '/till/products/:id').answer(ctx({ params: { id: '4' } }))).toMatchObject({ name: 'Panadol' })
    expect(route('GET', '/till/products/:id/lots').answer(ctx({ params: { id: '4' } }))).toEqual([{ batchId: 1 }])
  })
  it('a credit payment queues with its cash and the balance shown', () => {
    const r = route('POST', '/till/customers/:id/payment')
    const c = ctx({ params: { id: '1' }, body: { amount: '300', method: 'cash' } })
    expect(r.queue).toBe(true)
    expect(r.call!(c)).toEqual({ cashIn: '300', saw: { customerId: 1, balance: '900.00' } })
    expect(r.answer(c)).toMatchObject({ paymentId: 'temp:r1' })
  })
})
