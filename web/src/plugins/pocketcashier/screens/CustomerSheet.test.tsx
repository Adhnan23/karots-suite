import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useReducer } from 'react'
import { afterEach, describe, expect, it } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState, tillReducer } from '@/features/till/cart/store'
import { Pay } from './Pay'

const tv = {
  id: 1, name: 'TV', nameLocal: '', barcode: '', sellingPrice: '1000.00', wholesalePrice: '0.00', taxRate: '0', stockQty: '5',
  unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial: false, warrantyMonths: 0, lowStock: false, emoji: '',
  badges: [], packs: [], categoryId: 1, barcodes: [], packCodes: [],
} as unknown as CopyProduct
const copy = {
  symbol: 'Rs.', priceOptions: {}, products: [tv], places: [], tenderMethods: [], denominations: [], defaultSaleType: 'retail',
  customers: [{ id: 4, name: 'Nimal Perera', phone: '0771234567', balance: '300.00', creditLimit: '500.00' }],
} as unknown as TillCopy

function Harness() {
  const [cart, dispatch] = useReducer(tillReducer, tillReducer(initialState('retail'), { type: 'add', product: tv, lots: [] }))
  return <Pay who={{ id: 7, name: 'Cashier' } as never} copy={copy} cart={cart} dispatch={dispatch} live={false} onBack={() => undefined} />
}

describe('customers on Pay', () => {
  afterEach(cleanup)

  it('finds a customer by phone and shows their balance and limit', () => {
    render(<Harness />)
    fireEvent.click(screen.getByRole('button', { name: /Walk-in/ }))
    fireEvent.change(screen.getByLabelText('Find a customer'), { target: { value: '0771' } })
    fireEvent.click(screen.getByRole('button', { name: /Nimal Perera/ }))
    expect(screen.getByText(/owes Rs\. 300\.00 · limit Rs\. 500\.00/)).toBeTruthy()
  })

  it('warns over the limit on account, and Finish stays on', () => {
    render(<Harness />)
    fireEvent.click(screen.getByRole('button', { name: /Walk-in/ }))
    fireEvent.change(screen.getByLabelText('Find a customer'), { target: { value: 'nimal' } })
    fireEvent.click(screen.getByRole('button', { name: /Nimal Perera/ }))
    fireEvent.click(screen.getByRole('button', { name: 'On account' }))
    fireEvent.change(screen.getByLabelText('Amount tendered'), { target: { value: '1000' } })
    expect(screen.getByText(/over the Rs\. 500\.00 limit/)).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Finish sale' }) as HTMLButtonElement).disabled).toBe(false)
  })

  it('says a new customer needs the PC when it is not reachable', () => {
    render(<Harness />)
    fireEvent.click(screen.getByRole('button', { name: /Walk-in/ }))
    fireEvent.click(screen.getByRole('button', { name: '+ New customer' }))
    expect(screen.getByRole('alert').textContent).toMatch(/add the customer at the counter/)
  })
})
