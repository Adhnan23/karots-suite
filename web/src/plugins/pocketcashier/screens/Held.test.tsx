import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState, tillReducer } from '@/features/till/cart/store'
import { Held } from './Held'

const sendNow = vi.fn(async (_a: unknown) => ({ ref: 'r', status: 'applied' }))
vi.mock('../sync', () => ({ sendNow: (a: unknown) => sendNow(a) }))

const copy = {
  symbol: 'Rs.', defaultSaleType: 'retail',
  held: [{ id: 30, userId: 7, label: 'Ravi', saleType: 'retail', customerId: 0, discount: '0', cart: '{}', itemCount: 1, total: '90.00', createdAt: '' }],
} as unknown as TillCopy
const tv = { id: 1, name: 'TV', sellingPrice: '100.00', wholesalePrice: '0', packs: [], unitAllowDecimal: false, stockQty: '5', taxRate: '0' } as unknown as CopyProduct

describe('held sales on the phone', () => {
  afterEach(() => { cleanup(); sendNow.mockClear() })

  it('removes a held sale after asking, as the PC does', async () => {
    render(<Held who={{ id: 7 } as never} copy={copy} out={[]} cart={initialState('retail')} dispatch={() => {}} onSent={() => {}} onResumed={() => {}} />)
    fireEvent.click(screen.getByRole('button', { name: 'Remove held sale Ravi' }))
    expect(sendNow).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Yes, remove' }))
    await waitFor(() => expect(sendNow).toHaveBeenCalledWith({ kind: 'unhold', userId: 7, body: { heldId: 30 } }))
  })

  it('clears the sale on screen after asking', () => {
    const seen: unknown[] = []
    const cart = tillReducer(initialState('retail'), { type: 'add', product: tv, lots: [] })
    render(<Held who={{ id: 7 } as never} copy={copy} out={[]} cart={cart} dispatch={(a) => seen.push(a)} onSent={() => {}} onResumed={() => {}} />)
    fireEvent.click(screen.getByRole('button', { name: 'Clear sale' }))
    fireEvent.click(screen.getByRole('button', { name: 'Yes, clear it' }))
    expect(seen).toMatchObject([{ type: 'newSale' }])
  })
})
