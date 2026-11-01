import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useReducer } from 'react'
import { afterEach, describe, expect, it } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState, tillReducer } from '@/features/till/cart/store'
import { saleTotal } from '@/features/till/cart/totals'
import type { TillState } from '@/features/till/cart/types'
import { CartSummary } from './CartSummary'

const tv = {
  id: 1, name: 'TV', nameLocal: '', barcode: '', sellingPrice: '1000.00', wholesalePrice: '800.00', taxRate: '0', stockQty: '5',
  unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial: false, warrantyMonths: 0, lowStock: false, emoji: '',
  badges: [], packs: [], categoryId: 1, barcodes: [], packCodes: [],
} as unknown as CopyProduct
const copy = { symbol: 'Rs.', priceOptions: {}, products: [tv], customers: [] } as unknown as TillCopy

function Harness({ seen }: { seen: (s: TillState) => void }) {
  const [cart, dispatch] = useReducer(tillReducer, tillReducer(initialState('retail'), { type: 'add', product: tv, lots: [] }))
  seen(cart)
  return <CartSummary cart={cart} dispatch={dispatch} copy={copy} />
}

function open() {
  let state = initialState('retail')
  render(<Harness seen={(s) => { state = s }} />)
  return () => state
}

describe('the cart summary', () => {
  afterEach(cleanup)

  it('takes 10% off the bill, shown as the till totals it', () => {
    const state = open()
    fireEvent.change(screen.getByLabelText('Bill discount'), { target: { value: '10' } })
    fireEvent.click(screen.getByRole('button', { name: '%' }))
    expect(saleTotal(state())).toBe(900)
    expect(screen.getByLabelText('Total').textContent).toContain('900.00')
  })

  it('re-prices a line at its wholesale price when the sale turns wholesale', () => {
    const state = open()
    fireEvent.click(screen.getByRole('button', { name: 'Wholesale' }))
    expect(state().saleType).toBe('wholesale')
    expect(saleTotal(state())).toBe(800)
  })
})
