import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useReducer } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
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
  people: [{ id: 7, name: 'Cashier', canManageCredit: true }, { id: 8, name: 'Other', canManageCredit: false }],
  customers: [{ id: 4, name: 'Nimal Perera', phone: '0771234567', balance: '300.00', creditLimit: '500.00' }],
} as unknown as TillCopy

function Harness({ who = 7, live = false }: { who?: number; live?: boolean }) {
  const [cart, dispatch] = useReducer(tillReducer, tillReducer(initialState('retail'), { type: 'add', product: tv, lots: [] }))
  return <Pay who={{ id: who, name: 'Cashier' } as never} copy={copy} cart={cart} dispatch={dispatch} live={live} onBack={() => undefined} />
}

describe('customers on Pay', () => {
  afterEach(() => { cleanup(); vi.unstubAllGlobals() })

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

  it("puts what is still to pay on the customer's account in one tap (the PC's shortfall prompt)", () => {
    render(<Harness />)
    fireEvent.click(screen.getByRole('button', { name: /Walk-in/ }))
    fireEvent.change(screen.getByLabelText('Find a customer'), { target: { value: 'nimal' } })
    fireEvent.click(screen.getByRole('button', { name: /Nimal Perera/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Cash' }))
    fireEvent.change(screen.getByLabelText('Amount tendered'), { target: { value: '400' } })
    fireEvent.click(screen.getByRole('button', { name: 'Put Rs. 600.00 on account' }))
    expect(screen.getByText('On account: Rs. 600.00')).toBeTruthy()
    expect(screen.queryByText('Still to pay')).toBeNull()
  })

  it('says a new customer needs the PC when it is not reachable', () => {
    render(<Harness />)
    fireEvent.click(screen.getByRole('button', { name: /Walk-in/ }))
    fireEvent.click(screen.getByRole('button', { name: '+ New customer' }))
    expect(screen.getByRole('alert').textContent).toMatch(/add the customer at the counter/)
  })

  it('lets a person who manages credit adjust the limit (on the PC)', async () => {
    const fetch = vi.fn(async () => new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetch)
    render(<Harness live />)
    fireEvent.click(screen.getByRole('button', { name: /Walk-in/ }))
    fireEvent.change(screen.getByLabelText('Find a customer'), { target: { value: 'nimal' } })
    fireEvent.click(screen.getByRole('button', { name: /Nimal Perera/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Adjust credit limit' }))
    fireEvent.change(screen.getByLabelText('New credit limit'), { target: { value: '2000' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/till/customers/4/credit-limit', expect.objectContaining({ method: 'PATCH', body: '{"creditLimit":"2000"}' })))
  })

  it('never offers it to someone who does not manage credit', () => {
    render(<Harness who={8} live />)
    fireEvent.click(screen.getByRole('button', { name: /Walk-in/ }))
    fireEvent.change(screen.getByLabelText('Find a customer'), { target: { value: 'nimal' } })
    fireEvent.click(screen.getByRole('button', { name: /Nimal Perera/ }))
    expect(screen.queryByRole('button', { name: 'Adjust credit limit' })).toBeNull()
  })
})
