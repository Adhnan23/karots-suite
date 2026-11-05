import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { TillCopy, WarrantyReceiptView, WarrantyUnitRow } from '@/api/types.gen'
import type { Action } from '../../store/outbox'
import { Warranty } from './Warranty'

const sendCall = vi.fn()
vi.mock('../../sync', () => ({ sendCall: (...a: unknown[]) => sendCall(...a) }))
const printWarrantySlip = vi.fn(async () => {})
vi.mock('@/features/till/print', async (orig) => ({ ...(await orig<object>()), printWarrantySlip: (...a: unknown[]) => printWarrantySlip(...(a as [])) }))
const findUnit = vi.fn()
const findSales = vi.fn()
vi.mock('../../store/recent', () => ({ findUnit: (...a: unknown[]) => findUnit(...a), findSales: (...a: unknown[]) => findSales(...a) }))
const api = vi.fn()
vi.mock('@/api/client', async (orig) => ({ ...(await orig<object>()), api: (...a: unknown[]) => api(...a) }))

const unit = { id: 3, serial: 'SN-77', productName: 'Nokia', customerName: 'Sunil', receiptNo: 'S-5', soldAt: '2026-11-01T10:00:00Z', months: 12, until: '2027-11-01', left: '11 mo left', status: 'active', underWarranty: true } as WarrantyUnitRow
const receipt = { saleId: 5, receiptNo: 'S-5', soldAt: '2026-11-01T10:00:00Z', customerName: 'Sunil', lines: [{ productId: 9, name: 'Kettle', unitAbbr: 'pc', qty: '2', replaced: 0, remaining: '2', months: 6, until: '2027-05-01', underWarranty: true }] } as WarrantyReceiptView
const copy = { symbol: 'Rs.', askToPrint: false } as unknown as TillCopy
const who = { id: 7, name: 'Nimal' } as never

function open(live = false, out: Action[] = []) {
  render(<Warranty who={who} copy={copy} out={out} live={live} onSent={() => {}} onBack={() => {}} />)
}
const look = (text: string) => {
  fireEvent.change(screen.getByLabelText('Serial or receipt'), { target: { value: text } })
  fireEvent.click(screen.getByRole('button', { name: 'Find' }))
}

describe('Warranty on the phone', () => {
  beforeEach(() => {
    sendCall.mockReset(); printWarrantySlip.mockClear(); api.mockReset()
    findUnit.mockReset().mockResolvedValue(unit)
    findSales.mockReset().mockResolvedValue([])
  })
  afterEach(cleanup)

  it('finds a unit by serial offline', async () => {
    open()
    look('SN-77')
    expect(await screen.findByText('Nokia')).toBeTruthy()
    expect(screen.getByText(/Covered/)).toBeTruthy()
    expect(api).not.toHaveBeenCalled()
  })

  it('offline, saves a swap as a call', async () => {
    sendCall.mockResolvedValue(null)
    open()
    look('SN-77')
    await screen.findByText('Nokia')
    fireEvent.change(screen.getByLabelText('New unit serial'), { target: { value: 'SN-88' } })
    fireEvent.click(screen.getByRole('button', { name: 'Dead' }))
    fireEvent.click(screen.getByRole('button', { name: /Swap/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0]).toEqual([7, expect.objectContaining({ method: 'POST', path: '/api/v1/till/warranty/replace',
      body: { unitId: 3, newSerial: 'SN-88', reason: 'Dead' }, label: expect.stringContaining('SN-77') })])
    expect(await screen.findByRole('status')).toHaveProperty('textContent', expect.stringMatching(/sends when the PC is back/))
  })

  it('a swap waiting on this phone blocks a second of the same unit', async () => {
    const waiting: Action = { ref: 'r', kind: 'call', userId: 7, at: '', body: { method: 'POST', path: '/api/v1/till/warranty/replace', label: '', body: { unitId: 3, newSerial: 'SN-88' } } }
    open(false, [waiting])
    look('SN-77')
    await screen.findByText('Nokia')
    expect(screen.queryByRole('button', { name: /Swap/ })).toBeNull()
    expect(screen.getByText(/already swapped on this phone/)).toBeTruthy()
  })

  it('out of warranty, no swap is offered', async () => {
    findUnit.mockResolvedValue({ ...unit, underWarranty: false, left: 'expired' })
    open()
    look('SN-77')
    await screen.findByText(/Out of warranty/)
    expect(screen.queryByRole('button', { name: /Swap/ })).toBeNull()
  })

  it('finds by receipt offline, and saves a like-for-like replacement', async () => {
    findUnit.mockResolvedValue(null)
    findSales.mockResolvedValue([{ receiptNo: 'S-5', warranty: { units: [], receipt } }])
    sendCall.mockResolvedValue(null)
    open()
    look('S-5')
    await screen.findByText('Kettle')
    fireEvent.click(screen.getByRole('button', { name: /Replace/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0][1]).toMatchObject({ path: '/api/v1/till/warranty/receipt-replace', body: { saleId: 5, productId: 9 } })
  })

  it('live, asks the PC, and prints the swap slip', async () => {
    api.mockResolvedValue({ ...unit, claims: [] })
    sendCall.mockResolvedValue({ ref: 'r', status: 'applied', resultId: 41 })
    open(true)
    look('SN-77')
    await screen.findByText('Nokia')
    expect(api).toHaveBeenCalledWith('/till/warranty/serial/SN-77')
    fireEvent.change(screen.getByLabelText('New unit serial'), { target: { value: 'SN-88' } })
    fireEvent.click(screen.getByRole('button', { name: /Swap/ }))
    await waitFor(() => expect(printWarrantySlip).toHaveBeenCalledWith(41))
  })

  it('offline and not found, says come back when the PC is on', async () => {
    findUnit.mockResolvedValue(null)
    open()
    look('OLD')
    expect(await screen.findByText(/last 30 days.*when the PC is on/)).toBeTruthy()
  })
})
