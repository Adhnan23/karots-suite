import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import { Convert } from './Convert'
import { WriteOff } from './WriteOff'

const sendCall = vi.fn()
vi.mock('../../sync', () => ({ sendCall: (...a: unknown[]) => sendCall(...a) }))

const copy = { symbol: 'Rs.', priceOptions: { 4: [{ batchId: 70, batchNo: 'A1', qtyRemaining: '4', price: '100.00', expiryDate: '2027-01-01' }, { batchId: 71, batchNo: 'B2', qtyRemaining: '6', price: '110.00', expiryDate: '' }] },
  products: [{ id: 4, name: 'Panadol', unitAbbr: 'pc', stockQty: '10', isService: false, sellingPrice: '100', barcode: '', barcodes: [], categoryId: 1, badges: [] }, { id: 8, name: 'Copy job', unitAbbr: 'pc', stockQty: '0', isService: true, barcodes: [], categoryId: 1, badges: [] }],
  conversions: [{ id: 9, fromId: 4, toId: 5, fromName: 'Sugar sack', toName: 'Sugar 1kg', fromUnit: 'sack', toUnit: 'kg', ratio: '25', fromStock: '2', toStock: '3', note: '' }] } as unknown as TillCopy
const who = { id: 7, name: 'Nimal' } as never
const props = { who, copy, out: [], live: false, onSent: () => {}, onBack: () => {} }

describe('Write-off on the phone', () => {
  beforeEach(() => sendCall.mockReset().mockResolvedValue(null))
  afterEach(cleanup)
  const pick = () => {
    fireEvent.change(screen.getByLabelText('Find a product'), { target: { value: 'pan' } })
    fireEvent.click(screen.getByRole('button', { name: /Panadol/ }))
  }

  it('saves a call offline, oldest lot first by default', async () => {
    render(<WriteOff {...props} />)
    pick()
    fireEvent.change(screen.getByLabelText('Quantity'), { target: { value: '2' } })
    fireEvent.click(screen.getByRole('button', { name: /Take 2 pc out/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0]).toEqual([7, expect.objectContaining({ method: 'POST', path: '/api/v1/till/write-off', label: expect.stringContaining('Panadol'),
      body: { productId: 4, batchId: 0, quantity: '2', note: '', reason: 'damage' } })])
    expect(await screen.findByRole('status')).toHaveProperty('textContent', expect.stringMatching(/sends when the PC is back/))
  })

  it('can take a chosen lot, with another reason', async () => {
    render(<WriteOff {...props} />)
    pick()
    fireEvent.click(screen.getByRole('radio', { name: /Shop use/ }))
    fireEvent.click(screen.getByRole('radio', { name: /B2/ }))
    fireEvent.click(screen.getByRole('button', { name: /Take 1 pc out/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0][1].body).toMatchObject({ batchId: 71, reason: 'own_use' })
  })

  it('beyond the stock shown still saves, with a warning', async () => {
    render(<WriteOff {...props} />)
    pick()
    fireEvent.change(screen.getByLabelText('Quantity'), { target: { value: '50' } })
    expect(screen.getByText(/more than the 10 shown/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /Take 50 pc out/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
  })

  it('services have no stock to take out', () => {
    render(<WriteOff {...props} />)
    fireEvent.change(screen.getByLabelText('Find a product'), { target: { value: 'copy' } })
    expect(screen.queryByRole('button', { name: /Copy job/ })).toBeNull()
  })

  it('shows what the PC refused', async () => {
    sendCall.mockResolvedValue({ ref: 'r', status: 'failed', note: 'not enough stock' })
    render(<WriteOff {...{ ...props, live: true }} />)
    pick()
    fireEvent.click(screen.getByRole('button', { name: /Take 1 pc out/ }))
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'not enough stock')
  })
})

describe('Convert on the phone', () => {
  beforeEach(() => sendCall.mockReset().mockResolvedValue(null))
  afterEach(cleanup)

  it('saves a call offline, and sends what came out only when it differs', async () => {
    render(<Convert {...props} />)
    fireEvent.click(screen.getByRole('button', { name: /Sugar sack/ }))
    fireEvent.click(screen.getByRole('button', { name: /Open 1 sack/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalledTimes(1))
    expect(sendCall.mock.calls[0][1]).toMatchObject({ path: '/api/v1/till/conversions/9/run', body: { quantity: '1' } })
    expect(sendCall.mock.calls[0][1].body.toQty).toBeUndefined()
    expect(await screen.findByRole('status')).toHaveProperty('textContent', expect.stringMatching(/sends when the PC is back/))
  })

  it('records what really came out', async () => {
    render(<Convert {...props} />)
    fireEvent.click(screen.getByRole('button', { name: /Sugar sack/ }))
    fireEvent.change(screen.getByLabelText('Got'), { target: { value: '24.5' } })
    fireEvent.click(screen.getByRole('button', { name: /get 24\.5 kg/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0][1].body).toEqual({ quantity: '1', toQty: '24.5' })
  })

  it('more than the stock shown still saves, with a warning', async () => {
    render(<Convert {...props} />)
    fireEvent.click(screen.getByRole('button', { name: /Sugar sack/ }))
    fireEvent.change(screen.getByLabelText('Packs to open'), { target: { value: '5' } })
    expect(screen.getByText(/more than the 2 shown/)).toBeTruthy()
    expect(screen.getByRole('button', { name: /Open 5 sack/ })).toHaveProperty('disabled', false)
  })
})
