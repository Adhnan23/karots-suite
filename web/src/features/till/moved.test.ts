import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { holdOnMove, MOVED_LABEL, toHeldBlob } from './cart/held'
import { initialState, tillReducer } from './cart/store'

vi.mock('@/api/client', () => ({ api: vi.fn(() => Promise.resolve({})) }))

describe('holdOnMove', () => {
  beforeEach(() => vi.mocked(api).mockClear())
  it('parks a cart with lines once, labelled', async () => {
    let s = initialState('retail')
    s = tillReducer(s, { type: 'addService', line: { productId: 3, name: 'Job', price: 50 } })
    s = tillReducer(s, { type: 'addService', line: { productId: 4, name: 'Fix', price: 25 } })
    await holdOnMove(s)
    expect(api).toHaveBeenCalledTimes(1)
    const [path, opts] = vi.mocked(api).mock.calls[0] as [string, { json: { label: string; cart: string; itemCount: number } }]
    expect(path).toBe('/till/held')
    expect(opts.json.label).toBe(MOVED_LABEL)
    expect(opts.json.itemCount).toBe(2)
    expect(JSON.parse(opts.json.cart).lines).toHaveLength(2)
  })
  it('posts nothing for an empty cart', async () => {
    await holdOnMove(initialState('retail'))
    await holdOnMove(null)
    expect(api).not.toHaveBeenCalled()
  })
})

describe('a ticket', () => {
  it('a hold or a ticket keeps the payment rows a plugin put on Pay, not typed cash', () => {
    let s = initialState('retail')
    s = { ...s, tenders: [{ method: 'credit', amount: '500', reference: '', accountId: '', meta: {} }, { method: 'cash', amount: '100', reference: '', accountId: '', meta: {} }] }
    const held = (cart: string) => ({ id: 1, label: '', saleType: 'retail', discount: '0', cart, itemCount: 0, total: '0', createdAt: '' })
    expect(tillReducer(initialState('retail'), { type: 'resume', held: held(toHeldBlob(s)) }).tenders.map((t) => t.method)).toEqual(['credit'])
  })
})
