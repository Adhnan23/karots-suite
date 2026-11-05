import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState, tillReducer } from '@/features/till/cart/store'
import { slots } from '@/plugins/slots'
import { Pay } from './Pay'

const sendNow = vi.fn()
vi.mock('../sync', async (orig) => ({ ...(await orig<object>()), sendNow: (...a: unknown[]) => sendNow(...a) }))

const product = { id: 1, name: 'Tea', sellingPrice: '100.00', wholesalePrice: '0', stockQty: '5', unitAbbr: 'pc', isService: false, packs: [] } as unknown as CopyProduct
const copy = { symbol: 'Rs.', askToPrint: true, places: [], tenderMethods: [], denominations: [], defaultSaleType: 'retail', customers: [], products: [], people: [] } as unknown as TillCopy
const cart = tillReducer(initialState('retail'), { type: 'add', product, lots: [] })
const who = { id: 7, name: 'Nimal', role: 'cashier' }

describe('finishing a sale on the phone', () => {
  afterEach(() => { cleanup(); slots.tillAfterSale.pop() })

  it('offline, a plugin\'s after-sale call refers to the sale by its temporary reference', async () => {
    const hook = vi.fn(async () => {})
    slots.tillAfterSale.push(hook)
    sendNow.mockResolvedValue(null)
    render(<Pay who={who} copy={copy} cart={cart} dispatch={() => {}} live={false} onBack={() => {}} />)
    fireEvent.click(screen.getByRole('button', { name: 'Finish sale' }))
    await waitFor(() => expect(hook).toHaveBeenCalled())
    const ref = sendNow.mock.calls[0][0].ref
    expect((hook.mock.calls[0] as unknown[])[0]).toMatchObject({ saleId: `temp:${ref}`, lines: cart.lines })
  })

  it('online, it gets the real sale id', async () => {
    const hook = vi.fn(async () => {})
    slots.tillAfterSale.push(hook)
    sendNow.mockResolvedValue({ ref: 'r', status: 'applied', saleId: 55, receiptNo: 'S-55' })
    render(<Pay who={who} copy={copy} cart={cart} dispatch={() => {}} live onBack={() => {}} />)
    fireEvent.click(screen.getByRole('button', { name: 'Finish sale' }))
    await waitFor(() => expect(hook).toHaveBeenCalled())
    expect((hook.mock.calls[0] as unknown[])[0]).toMatchObject({ saleId: 55 })
  })
})
