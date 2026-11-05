import { describe, expect, it } from 'vitest'
import type { Action } from '../../store/outbox'
import type { RouteCtx } from '../fetch'
import { rechargeRoutes } from './recharge'

const part = { carriers: [], floats: [{ id: 3, label: 'Dialog', balance: '1000.00' }, { id: 4, label: 'Mobitel', balance: '50.00' }], packages: [], banks: [], billTypes: [], bills: [], products: { billpay: 1, deposit: 2, cashout: 3 } }
const ctx = (over: Partial<RouteCtx> = {}): RouteCtx => ({ parts: { recharge: part }, copy: null, params: {}, query: new URLSearchParams(), body: undefined, ref: 'r1', out: [], userId: 7, ...over })
const route = (m: string, p: string) => rechargeRoutes.find((r) => r.method === m && r.path === p)!
const reload = (device: number, price: string): Action => ({ ref: 's', kind: 'sale', userId: 7, at: '', body: { items: [{ quantity: '1', priceOverride: price, plugin: { recharge: { kind: 'reload', device } } }] } })

describe('Reload & Bills with the PC down', () => {
  it('answers the till\'s data from the part', () => {
    expect(route('GET', '/x/recharge/till').answer(ctx())).toMatchObject({ products: { billpay: 1 } })
  })
  it('a waiting reload lowers the float shown', () => {
    const a = route('GET', '/x/recharge/till').answer(ctx({ out: [reload(3, '100')] })) as typeof part
    expect(a.floats.map((f) => f.balance)).toEqual(['900.00', '50.00'])
  })
  it('a waiting money-in (cash out) raises it', () => {
    const cashout: Action = { ref: 's', kind: 'sale', userId: 7, at: '', body: { items: [{ quantity: '1', priceOverride: '200', payout: true, plugin: { recharge: { kind: 'cashout', device: 4 } } }] } }
    const a = route('GET', '/x/recharge/till').answer(ctx({ out: [cashout] })) as typeof part
    expect(a.floats[1].balance).toBe('250.00')
  })
  it('a float may go below zero: the money already moved in the carrier\'s app', () => {
    const a = route('GET', '/x/recharge/till').answer(ctx({ out: [reload(4, '100')] })) as typeof part
    expect(a.floats[1].balance).toBe('-50.00')
  })
  it('with nothing stored yet the screen stays empty, not broken', () => {
    expect(route('GET', '/x/recharge/till').answer(ctx({ parts: {} }))).toBeNull()
  })
  it('old sale lines aren\'t on the phone: no float field on a return', () => {
    expect(route('GET', '/x/recharge/sale-lines').answer(ctx())).toEqual([])
  })
  it('a quick package queues, with a temporary id for the cart line', () => {
    const r = route('POST', '/x/recharge/packages/quick')
    const a = r.answer(ctx({ body: { carrierId: 1, name: '1GB', price: '199', commission: '10' } })) as { package: { id: string; name: string; price: string }; created: boolean }
    expect(r.queue).toBe(true)
    expect(a).toMatchObject({ package: { id: 'temp:r1', name: '1GB', price: '199' }, created: true })
    expect(r.label!(ctx({ body: { name: '1GB' } }))).toContain('1GB')
  })
  it('drawer counts queue, and answer with what\'s expected', () => {
    const r = route('POST', '/x/recharge/counts/drawer')
    expect(r.queue).toBe(true)
    expect(r.answer(ctx({ body: { counts: [{ deviceId: 3, counted: '950' }] } }))).toEqual([{ deviceId: 3, expected: '1000.00', counted: '950' }])
  })
})
