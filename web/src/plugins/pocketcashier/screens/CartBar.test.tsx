import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState, tillReducer } from '@/features/till/cart/store'
import { CartLines } from './CartBar'

const tv = {
  id: 1, name: 'TV', sellingPrice: '100.00', wholesalePrice: '0', packs: [], unitAllowDecimal: false, stockQty: '1', taxRate: '0',
  isService: false, trackSerial: false, unitAbbr: 'pcs', badges: [],
} as unknown as CopyProduct
const copy = { symbol: 'Rs.', priceOptions: {}, products: [tv] } as unknown as TillCopy

describe('the phone cart', () => {
  afterEach(cleanup)

  it('notes a line over the count, without stopping the sale (the server flags it)', () => {
    let cart = tillReducer(initialState('retail'), { type: 'add', product: tv, lots: [] })
    cart = tillReducer(cart, { type: 'setQty', key: cart.lines[0].key, qty: 3, lots: [] })
    render(<CartLines cart={cart} dispatch={() => {}} copy={copy} />)
    expect(screen.getByText(/count shows 1/)).toBeTruthy()
  })
})
