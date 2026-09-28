import { beforeEach, describe, expect, it } from 'vitest'
import { loadCart, saveCart } from './persist'
import { initialState, tillReducer } from './store'

describe('cart persistence (auto-lock / reload safety)', () => {
  beforeEach(() => sessionStorage.clear())
  it('restores the same user’s cart', () => {
    const s = tillReducer(initialState('retail'), { type: 'addService', line: { productId: 1, name: 'x', price: 5 } })
    saveCart(7, s)
    expect(loadCart(7)?.lines).toHaveLength(1)
  })
  it('never shows one user’s cart to another', () => {
    saveCart(7, tillReducer(initialState('retail'), { type: 'addService', line: { productId: 1, name: 'x', price: 5 } }))
    expect(loadCart(8)).toBeNull()
  })
  it('ignores corrupt storage and drops transient notices', () => {
    sessionStorage.setItem('ks-till-cart-9', '{bad')
    expect(loadCart(9)).toBeNull()
    saveCart(9, { ...initialState('retail'), notice: { tone: 'error', text: 'x' } })
    expect(loadCart(9)?.notice).toBeNull()
  })
})
