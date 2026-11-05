import { describe, expect, it, vi } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState } from '@/features/till/cart/store'
import { phoneTillApi } from './tillApi'

const product = { id: 4, name: 'Panadol' } as CopyProduct
const copy = { products: [product], priceOptions: {} } as unknown as TillCopy

function make(over = {}) {
  const dispatch = vi.fn()
  const cart = { ...initialState('retail'), customerId: 3, ...over }
  return { dispatch, till: phoneTillApi(cart, dispatch, copy) }
}

describe('the plugins\' till, on the phone', () => {
  it('reads the cart', () => {
    const { till } = make({ meta: { a: 1 } })
    expect(till).toMatchObject({ saleType: 'retail', customerId: 3, saleMeta: { a: 1 } })
  })
  it('adds a service line, group and payout through the till\'s own reducer', () => {
    const { till, dispatch } = make()
    const line = { name: 'x' } as never
    till.addServiceLine(line)
    expect(dispatch).toHaveBeenLastCalledWith({ type: 'addService', line })
    till.addServiceGroup({ label: 'g', lines: [] })
    expect(dispatch).toHaveBeenLastCalledWith({ type: 'addServiceGroup', group: { label: 'g', lines: [] } })
    till.addPayout(line)
    expect(dispatch).toHaveBeenLastCalledWith({ type: 'addPayout', line })
  })
  it('adds a product from the copy', async () => {
    const { till, dispatch } = make()
    expect(await till.addProduct(4, { units: 2 })).toBeNull()
    expect(dispatch).toHaveBeenCalledWith(expect.objectContaining({ type: 'add', product, units: 2 }))
  })
  it('says so for a product it doesn\'t have', async () => {
    const { till, dispatch } = make()
    expect(await till.addProduct(99)).toMatch(/isn't on this phone/)
    expect(dispatch).not.toHaveBeenCalled()
  })
  it('adds a group of products from the copy', async () => {
    const { till, dispatch } = make()
    expect(await till.addGroup({ label: 'Deal', count: 1, items: [{ productId: 4, per: 1, discount: '0' }] })).toBeNull()
    expect(dispatch).toHaveBeenCalledWith({ type: 'addGroup', group: expect.objectContaining({ label: 'Deal', items: [{ product, per: 1, discount: '0' }] }) })
  })
  it('a group missing a product isn\'t added', async () => {
    const { till, dispatch } = make()
    expect(await till.addGroup({ label: 'Deal', count: 1, items: [{ productId: 99, per: 1, discount: '0' }] })).toMatch(/group wasn't added/)
    expect(dispatch).not.toHaveBeenCalled()
  })
  it('swaps a line', async () => {
    const { till, dispatch } = make()
    expect(await till.swapLine(1, 4, 2)).toBeNull()
    expect(dispatch).toHaveBeenCalledWith({ type: 'swap', key: 1, product, qty: 2 })
  })
  it('sets the customer, a tender, sale notes and a quantity', () => {
    const { till, dispatch } = make()
    till.setCustomer(9)
    expect(dispatch).toHaveBeenLastCalledWith({ type: 'customer', id: 9 })
    till.addTender('credit', '50')
    expect(dispatch).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'addTender', payment: expect.objectContaining({ method: 'credit', amount: '50' }) }))
    till.setSaleMeta('k', 1)
    expect(dispatch).toHaveBeenLastCalledWith({ type: 'saleMeta', key: 'k', value: 1 })
    till.setLineQty(2, 3)
    expect(dispatch).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'setQty', key: 2, qty: 3 }))
  })
  it('refreshDrawer is harmless', () => {
    expect(() => make().till.refreshDrawer()).not.toThrow()
  })
})
