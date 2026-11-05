import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import type { Action } from '../../store/outbox'
import { Receipts } from './Receipts'

const printSale = vi.fn(async () => {})
const printMoneySlip = vi.fn(async () => {})
vi.mock('@/features/till/print', async (orig) => ({ ...(await orig<object>()), printSale: (...a: unknown[]) => printSale(...(a as [])), printMoneySlip: (...a: unknown[]) => printMoneySlip(...(a as [])) }))
const sale = { id: 5, receiptNo: 'S-5', createdAt: '2026-11-04T10:00:00Z', customerName: 'Sunil', total: '300.00', lines: [{ id: 1, name: 'Panadol', quantity: '3', unitAbbr: 'pc', subtotal: '300.00', returnedQty: '0' }], payments: [{ method: 'cash', amount: '300.00' }] }
vi.mock('../../store/recent', () => ({ findSales: async () => [sale], getSale: async () => sale }))
const api = vi.fn()
vi.mock('@/api/client', async (orig) => ({ ...(await orig<object>()), api: (...a: unknown[]) => api(...a) }))

const copy = { symbol: 'Rs.', askToPrint: false, sessions: [] } as unknown as TillCopy
const who = { id: 7, name: 'Nimal' } as never
const waitingSale: Action = { ref: 'r1', kind: 'sale', userId: 7, at: '2026-11-05T09:00:00Z', body: { total: '120.00' } }
const waitingCall: Action = { ref: 'r2', kind: 'call', userId: 7, at: '2026-11-05T09:05:00Z', body: { method: 'POST', path: '/x', label: 'Credit from Sunil: Rs. 50.00' } }

function open(live = false, out: Action[] = []) {
  render(<Receipts who={who} copy={copy} out={out} live={live} onSent={() => {}} onBack={() => {}} />)
}

describe('Receipts on the phone', () => {
  beforeEach(() => { printSale.mockClear(); printMoneySlip.mockClear(); api.mockReset().mockImplementation(async (p: string) => (p.startsWith('/till/sales/') ? sale : [])) })
  afterEach(cleanup)

  it('shows sales from the copy, plus what waits on this phone', async () => {
    open(false, [waitingSale, waitingCall])
    expect(await screen.findByText('S-5')).toBeTruthy()
    expect(screen.getByText(/Sale.*Rs\. 120\.00/)).toBeTruthy()
    expect(screen.getByText('Credit from Sunil: Rs. 50.00')).toBeTruthy()
  })

  it('live, prints a slip at the counter at either width', async () => {
    open(true)
    fireEvent.click(await screen.findByText('S-5'))
    await screen.findByText('Panadol')
    fireEvent.click(screen.getByRole('button', { name: 'Print 58 mm' }))
    await waitFor(() => expect(printSale).toHaveBeenCalledWith(5, false, '58'))
    fireEvent.click(screen.getByRole('button', { name: 'Print 80 mm' }))
    await waitFor(() => expect(printSale).toHaveBeenCalledWith(5, false, '80'))
  })

  it('offline, a slip opens on screen with no Print button', async () => {
    open()
    fireEvent.click(await screen.findByText('S-5'))
    await screen.findByText('Panadol')
    expect(screen.queryByRole('button', { name: /Print/ })).toBeNull()
    expect(screen.getByText(/prints when the PC is on/i)).toBeTruthy()
  })

  it('live, the other slips come from the PC', async () => {
    api.mockImplementation(async (p: string) => (p.startsWith('/till/money-receipts') ? [{ id: 8, receiptNo: 'CR-8', kindLabel: 'Refund', party: 'Sunil', amount: '40.00', createdAt: '2026-11-04T10:00:00Z' }] : []))
    open(true)
    fireEvent.click(screen.getByRole('button', { name: 'Cash' }))
    fireEvent.click(await screen.findByRole('button', { name: /CR-8/ }))
    await waitFor(() => expect(printMoneySlip).toHaveBeenCalledWith(8))
  })

  it('offline, the other slips say they need the PC', async () => {
    open()
    fireEvent.click(screen.getByRole('button', { name: 'Cash' }))
    expect(await screen.findByText(/need the PC/)).toBeTruthy()
    expect(api).not.toHaveBeenCalled()
  })
})
