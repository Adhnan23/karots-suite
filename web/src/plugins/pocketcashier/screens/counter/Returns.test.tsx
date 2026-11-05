import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { SaleLine, SaleView, TillCopy } from '@/api/types.gen'
import type { Action } from '../../store/outbox'
import { Returns } from './Returns'

const sendCall = vi.fn()
vi.mock('../../sync', () => ({ sendCall: (...a: unknown[]) => sendCall(...a) }))
const printRefund = vi.fn(async () => {})
vi.mock('@/features/till/print', async (orig) => ({ ...(await orig<object>()), printRefund: (...a: unknown[]) => printRefund(...(a as [])) }))
const findSales = vi.fn()
const getSale = vi.fn()
vi.mock('../../store/recent', () => ({ findSales: (...a: unknown[]) => findSales(...a), getSale: (...a: unknown[]) => getSale(...a) }))
const api = vi.fn()
vi.mock('@/api/client', async (orig) => ({ ...(await orig<object>()), api: (...a: unknown[]) => api(...a) }))

const line = (over: Partial<SaleLine> = {}): SaleLine => ({ id: 11, name: 'Panadol', unitAbbr: 'pc', quantity: '3', returnedQty: '0', unitPrice: '100.00', subtotal: '300.00',
  refundUnit: '100.00', paid: '300.00', refundLeft: '300.00', isService: false, group: 0, groupLabel: '', groupSets: '', packLabel: '', ...over })
const sale = (over: Partial<SaleView> = {}): SaleView => ({ id: 5, receiptNo: 'S-5', createdAt: '2026-11-04T10:00:00Z', customerName: 'Walk-in', cashierName: 'Nimal', total: '300.00', status: 'completed',
  customerId: 0, subtotal: '300.00', discount: '0.00', tax: '0.00', paidAmount: '300.00', changeGiven: '0.00', owedOnSale: '0.00', lines: [line()], payments: [], flags: [], tillSource: '', ...over })
const copy = { symbol: 'Rs.', askToPrint: false, sessions: [{ userId: 7, expected: '1000' }] } as unknown as TillCopy
const who = { id: 7, name: 'Nimal' } as never

function open(live = false, out: Action[] = [], c = copy) {
  render(<Returns who={who} copy={c} out={out} live={live} onSent={() => {}} onBack={() => {}} />)
}
async function pickSale() {
  fireEvent.change(screen.getByLabelText('Find a sale'), { target: { value: 'S-5' } })
  fireEvent.click(await screen.findByText('S-5'))
  await screen.findByText('Panadol')
}

describe('Returns on the phone', () => {
  beforeEach(() => {
    sendCall.mockReset(); printRefund.mockClear(); api.mockReset()
    findSales.mockReset().mockResolvedValue([sale()])
    getSale.mockReset().mockResolvedValue(sale())
  })
  afterEach(cleanup)

  it('finds a sale from the copy offline, without asking the PC', async () => {
    open()
    await pickSale()
    expect(findSales).toHaveBeenCalled()
    expect(api).not.toHaveBeenCalled()
  })

  it('an older sale: asks the PC when online', async () => {
    findSales.mockResolvedValue([])
    api.mockImplementation(async (p: string) => (p.includes('?q=') ? [sale({ id: 90, receiptNo: 'S-90' })] : sale({ id: 90, receiptNo: 'S-90' })))
    open(true)
    fireEvent.change(screen.getByLabelText('Find a sale'), { target: { value: 'S-90' } })
    fireEvent.click(await screen.findByText('S-90'))
    await screen.findByText('Panadol')
    expect(api).toHaveBeenCalledWith('/till/sales/90')
  })

  it('offline, says too old and to come back when the PC is on', async () => {
    findSales.mockResolvedValue([])
    open()
    fireEvent.change(screen.getByLabelText('Find a sale'), { target: { value: 'S-90' } })
    expect(await screen.findByText(/last 30 days.*when the PC is on/)).toBeTruthy()
  })

  it('a return waiting on this phone lowers what can come back', async () => {
    const waiting: Action = { ref: 'r', kind: 'call', userId: 7, at: '', body: { method: 'POST', path: '/api/v1/till/sales/5/return', label: '', body: { lines: [{ saleItemId: 11, quantity: '2' }] } } }
    open(false, [waiting])
    await pickSale()
    expect(screen.getByText(/2 already back/)).toBeTruthy()
    fireEvent.click(screen.getByLabelText('More Panadol'))
    fireEvent.click(screen.getByLabelText('More Panadol'))
    expect(screen.getByRole('button', { name: /Refund Rs\. 100\.00 cash/ })).toBeTruthy()
  })

  it('offline, saves the return with the cash handed over', async () => {
    sendCall.mockResolvedValue(null)
    open()
    await pickSale()
    fireEvent.click(screen.getByLabelText('More Panadol'))
    fireEvent.click(screen.getByRole('button', { name: 'Wrong item' }))
    fireEvent.click(screen.getByRole('button', { name: /Refund Rs\. 100\.00 cash/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0]).toEqual([7, expect.objectContaining({
      method: 'POST', path: '/api/v1/till/sales/5/return', cashOut: '100.00', label: expect.stringContaining('S-5'),
      body: { reason: 'Wrong item', deferDrawerKick: true, lines: [{ saleItemId: 11, quantity: '1', disposition: 'restock' }] },
    })])
    expect(await screen.findByRole('status')).toHaveProperty('textContent', expect.stringMatching(/Give back Rs\. 100\.00.*sends when the PC is back/))
    expect(printRefund).not.toHaveBeenCalled()
  })

  it('comes off the account first, so no cash moves', async () => {
    sendCall.mockResolvedValue(null)
    getSale.mockResolvedValue(sale({ owedOnSale: '300.00', customerName: 'Sunil' }))
    open()
    await pickSale()
    fireEvent.click(screen.getByLabelText('More Panadol'))
    fireEvent.click(screen.getByRole('button', { name: 'Record the return' }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0][1].cashOut).toBeUndefined()
  })

  it('a faulty item is written off', async () => {
    sendCall.mockResolvedValue(null)
    open()
    await pickSale()
    fireEvent.click(screen.getByLabelText('More Panadol'))
    fireEvent.click(screen.getByLabelText(/Faulty/))
    fireEvent.click(screen.getByRole('button', { name: /Refund/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0][1].body.lines[0].disposition).toBe('damage')
  })

  it('a service can be refunded; a payout can\'t come back', async () => {
    getSale.mockResolvedValue(sale({ lines: [line({ id: 1, name: 'Photocopy', isService: true }), line({ id: 2, name: 'Bill payout', subtotal: '-500.00' })] }))
    open()
    fireEvent.change(screen.getByLabelText('Find a sale'), { target: { value: 'S-5' } })
    fireEvent.click(await screen.findByText('S-5'))
    await screen.findByText('Photocopy')
    expect(screen.getByLabelText('More Photocopy')).toBeTruthy()
    expect(screen.queryByLabelText('More Bill payout')).toBeNull()
  })

  it('online, prints the refund slip at the counter', async () => {
    sendCall.mockResolvedValue({ ref: 'r', status: 'applied' })
    api.mockImplementation(async (p: string) => (p.includes('?q=') ? [] : sale()))
    open(true)
    await pickSale()
    fireEvent.click(screen.getByLabelText('More Panadol'))
    fireEvent.click(screen.getByRole('button', { name: /Refund/ }))
    await waitFor(() => expect(printRefund).toHaveBeenCalledWith(5))
  })

  it('cash needs an open drawer', async () => {
    open(false, [], { ...copy, sessions: [] })
    await pickSale()
    fireEvent.click(screen.getByLabelText('More Panadol'))
    expect(screen.getByRole('button', { name: /Refund/ })).toHaveProperty('disabled', true)
    expect(screen.getByText(/Open the drawer first/)).toBeTruthy()
  })

  it('shows a plugin\'s field on a line coming back, and sends what it says', async () => {
    const { slots } = await import('@/plugins/slots')
    slots.returnLineFields.push({ key: 'recharge', Field: ({ onChange }) => <button onClick={() => onChange({ float: '50' })}>Float back</button> })
    sendCall.mockResolvedValue(null)
    open()
    await pickSale()
    expect(screen.queryByText('Float back')).toBeNull()
    fireEvent.click(screen.getByLabelText('More Panadol'))
    fireEvent.click(screen.getByText('Float back'))
    fireEvent.click(screen.getByRole('button', { name: /Refund/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0][1].body.lines[0].plugin).toEqual({ recharge: { float: '50' } })
    slots.returnLineFields.pop()
  })
})
