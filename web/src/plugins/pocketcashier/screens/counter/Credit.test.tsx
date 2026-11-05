import 'fake-indexeddb/auto'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import { Credit } from './Credit'

const sendCall = vi.fn()
vi.mock('../../sync', () => ({ sendCall: (...a: unknown[]) => sendCall(...a) }))
const printDebtSlip = vi.fn(async () => {})
vi.mock('@/features/till/print', async (orig) => ({ ...(await orig<object>()), printDebtSlip: (...a: unknown[]) => printDebtSlip(...(a as [])) }))

const copy = { symbol: 'Rs.', askToPrint: false, places: [], tenderMethods: [],
  sessions: [{ userId: 7, expected: '100' }],
  customers: [{ id: 1, name: 'Sunil', phone: '0771', balance: '900.00', creditLimit: '0' }, { id: 2, name: 'Mala', phone: '', balance: '-40.00', creditLimit: '0' }, { id: 3, name: 'Clear', phone: '', balance: '0', creditLimit: '0' }] } as unknown as TillCopy
const who = { id: 7, name: 'Nimal' } as never

function open(live = false, c = copy) {
  render(<Credit who={who} copy={c} out={[]} live={live} onSent={() => {}} onBack={() => {}} />)
}

describe('Credit on the phone', () => {
  beforeEach(() => { sendCall.mockReset(); printDebtSlip.mockClear() })
  afterEach(cleanup)

  it('lists who owes or holds an advance', () => {
    open()
    expect(screen.getByText('Sunil')).toBeTruthy()
    expect(screen.queryByText('Clear')).toBeNull()
    fireEvent.change(screen.getByLabelText('Find a customer'), { target: { value: 'mal' } })
    expect(screen.queryByText('Sunil')).toBeNull()
    expect(screen.getByText('Mala')).toBeTruthy()
  })

  it('offline, saves the payment and says it sends when the PC is back', async () => {
    sendCall.mockResolvedValue(null)
    open()
    fireEvent.click(screen.getByText('Sunil'))
    fireEvent.change(screen.getByLabelText('Amount'), { target: { value: '300' } })
    fireEvent.click(screen.getByRole('button', { name: /Collect/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0]).toEqual([7, expect.objectContaining({
      method: 'POST', path: '/api/v1/till/customers/1/payment', cashIn: '300',
      body: expect.objectContaining({ amount: '300', method: 'cash', deferDrawerKick: true }),
      saw: { customerId: 1, balance: '900.00' }, label: expect.stringContaining('Sunil'),
    })])
    expect(await screen.findByRole('status')).toHaveProperty('textContent', expect.stringMatching(/sends when the PC is back/))
    expect(printDebtSlip).not.toHaveBeenCalled()
  })

  it('more than they owe is allowed, and says the rest becomes an advance', () => {
    open()
    fireEvent.click(screen.getByText('Sunil'))
    fireEvent.change(screen.getByLabelText('Amount'), { target: { value: '1000' } })
    expect(screen.getByText(/Rs\. 100\.00 becomes an advance/)).toBeTruthy()
  })

  it('online, prints the slip at the counter', async () => {
    sendCall.mockResolvedValue({ ref: 'r', status: 'applied', resultId: 44 })
    open(true)
    fireEvent.click(screen.getByText('Sunil'))
    fireEvent.change(screen.getByLabelText('Amount'), { target: { value: '300' } })
    fireEvent.click(screen.getByRole('button', { name: /Collect/ }))
    await waitFor(() => expect(printDebtSlip).toHaveBeenCalledWith(44))
  })

  it('online and asked to print first, it asks', async () => {
    sendCall.mockResolvedValue({ ref: 'r', status: 'applied', resultId: 44 })
    open(true, { ...copy, askToPrint: true })
    fireEvent.click(screen.getByText('Sunil'))
    fireEvent.change(screen.getByLabelText('Amount'), { target: { value: '300' } })
    fireEvent.click(screen.getByRole('button', { name: /Collect/ }))
    expect(await screen.findByText('Print the receipt?')).toBeTruthy()
    expect(printDebtSlip).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Skip' }))
    expect(await screen.findByRole('status')).toHaveProperty('textContent', expect.stringMatching(/collected from Sunil/))
  })

  it('shows what the PC refused', async () => {
    sendCall.mockResolvedValue({ ref: 'r', status: 'failed', note: 'no such customer' })
    open(true)
    fireEvent.click(screen.getByText('Sunil'))
    fireEvent.change(screen.getByLabelText('Amount'), { target: { value: '300' } })
    fireEvent.click(screen.getByRole('button', { name: /Collect/ }))
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'no such customer')
  })

  it('an advance can be refunded, from the drawer', async () => {
    sendCall.mockResolvedValue(null)
    open()
    fireEvent.click(screen.getByText('Mala'))
    fireEvent.click(screen.getByRole('button', { name: 'Refund advance' }))
    fireEvent.click(screen.getByRole('button', { name: /Refund Rs\. 40\.00/ }))
    await waitFor(() => expect(sendCall).toHaveBeenCalled())
    expect(sendCall.mock.calls[0][1]).toMatchObject({ path: '/api/v1/till/customers/2/refund', cashOut: '40.00', body: { amount: '40.00', deferDrawerKick: true } })
  })

  it('cash needs an open drawer', () => {
    open(false, { ...copy, sessions: [] })
    fireEvent.click(screen.getByText('Sunil'))
    fireEvent.change(screen.getByLabelText('Amount'), { target: { value: '300' } })
    expect(screen.getByRole('button', { name: /Collect/ })).toHaveProperty('disabled', true)
    expect(screen.getByText(/Open the drawer first/)).toBeTruthy()
  })
})
