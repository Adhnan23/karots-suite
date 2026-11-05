import { describe, expect, it } from 'vitest'
import type { RouteCtx } from '../fetch'
import { combosRoutes } from './combos'
import { alternativesRoutes } from './alternatives'

const ctx = (parts: Record<string, unknown>, over: Partial<RouteCtx> = {}): RouteCtx => ({ parts, copy: null, params: {}, query: new URLSearchParams(), body: undefined, ref: 'r1', out: [], userId: 7, ...over })

describe('Combos with the PC down', () => {
  const route = (m: string, p: string) => combosRoutes.find((r) => r.method === m && r.path === p)!
  it('answers the list and the settings', () => {
    const parts = { combos: { till: [{ id: 1 }], settings: { autoApply: true } } }
    expect(route('GET', '/x/combos/till').answer(ctx(parts))).toEqual([{ id: 1 }])
    expect(route('GET', '/x/combos/settings').answer(ctx(parts))).toEqual({ autoApply: true })
  })
  it('with nothing stored yet, an empty list', () => {
    expect(route('GET', '/x/combos/till').answer(ctx({}))).toEqual([])
  })
  it('a sold set queues', () => {
    expect(route('POST', '/x/combos/sold').queue).toBe(true)
  })
})

describe('Alternatives with the PC down', () => {
  const r = alternativesRoutes[0]
  it('offers the group\'s products', () => {
    const parts = { alternatives: { byProduct: { 4: [{ productId: 5, name: 'Other brand' }] } } }
    expect(r.answer(ctx(parts, { params: { pid: '4' } }))).toEqual([{ productId: 5, name: 'Other brand' }])
  })
  it('a product with none: nothing', () => {
    expect(r.answer(ctx({ alternatives: { byProduct: {} } }, { params: { pid: '9' } }))).toEqual([])
  })
})
