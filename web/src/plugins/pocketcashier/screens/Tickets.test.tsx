import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { HeldSale } from '@/api/types.gen'
import { initialState } from '@/features/till/cart/store'
import { TicketsBeforeClose, TicketStrip, useTickets } from './Tickets'

const api = vi.fn()
vi.mock('@/api/client', async (orig) => ({ ...(await orig<object>()), api: (...a: unknown[]) => api(...a) }))

const ticket = (over: Partial<HeldSale> = {}) => ({ id: 3, ticketNo: 42, label: 'Blue shirt', total: '500.00', forMe: true, preparedByName: 'Kamal', forCashierName: 'Nimal',
  cart: JSON.stringify({ v: 2, lines: [{ name: 'Panadol', qty: 2 }], billDiscountType: 'fixed' }), saleType: 'retail', discount: '0', itemCount: 1, ...over }) as unknown as HeldSale

function Strip({ live, cartLines = 0, onOpen }: { live: boolean; cartLines?: number; onOpen: (t: HeldSale) => void }) {
  const tickets = useTickets(live)
  const cart = { ...initialState('retail'), lines: Array(cartLines).fill({}) }
  return <TicketStrip tickets={tickets} sym="Rs." cart={cart} onOpen={onOpen} />
}

describe('tickets on the phone', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    api.mockReset().mockResolvedValue([])
    vi.stubGlobal('AudioContext', class {
      destination = {}
      currentTime = 0
      createOscillator() { beeps.n++; return { connect: () => ({ connect: () => {} }), start() {}, stop() {}, onended: null } }
      createGain() { return {} }
      close() {}
    })
    beeps.n = 0
  })
  afterEach(() => { cleanup(); vi.useRealTimers(); vi.unstubAllGlobals() })
  const beeps = { n: 0 }

  it('a new ticket for this person shows and beeps while live', async () => {
    render(<Strip live onOpen={() => {}} />)
    await waitFor(() => expect(api).toHaveBeenCalledWith('/till/tickets'))
    api.mockResolvedValue([ticket()])
    await act(async () => { await vi.advanceTimersByTimeAsync(5_000) })
    expect(await screen.findByRole('button', { name: /Ticket 42, Blue shirt/ })).toBeTruthy()
    expect(beeps.n).toBe(1)
    await act(async () => { await vi.advanceTimersByTimeAsync(5_000) })
    expect(beeps.n).toBe(1)
  })

  it('what already waited at sign-in shows without a beep', async () => {
    api.mockResolvedValue([ticket()])
    render(<Strip live onOpen={() => {}} />)
    await screen.findByRole('button', { name: /Ticket 42/ })
    expect(beeps.n).toBe(0)
  })

  it('a ticket for another till shows, without a beep', async () => {
    render(<Strip live onOpen={() => {}} />)
    await waitFor(() => expect(api).toHaveBeenCalled())
    api.mockResolvedValue([ticket({ forMe: false })])
    await act(async () => { await vi.advanceTimersByTimeAsync(5_000) })
    await screen.findByRole('button', { name: /Ticket 42/ })
    expect(beeps.n).toBe(0)
  })

  it('offline shows no tickets, no error, and asks nothing', async () => {
    render(<Strip live={false} onOpen={() => {}} />)
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000) })
    expect(api).not.toHaveBeenCalled()
    expect(screen.queryByRole('group', { name: 'Tickets' })).toBeNull()
  })

  it('opening over a cart with items is refused, as on the PC', async () => {
    api.mockResolvedValue([ticket()])
    const onOpen = vi.fn()
    render(<Strip live cartLines={1} onOpen={onOpen} />)
    fireEvent.click(await screen.findByRole('button', { name: /Ticket 42/ }))
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'Finish or hold the current sale first')
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('opens it: the server hands it to this till, and the cart takes it', async () => {
    const t = ticket()
    api.mockImplementation(async (p: string) => (p === '/till/tickets' ? [t] : t))
    const onOpen = vi.fn()
    render(<Strip live onOpen={onOpen} />)
    fireEvent.click(await screen.findByRole('button', { name: /Ticket 42/ }))
    await waitFor(() => expect(onOpen).toHaveBeenCalledWith(t))
    expect(api).toHaveBeenCalledWith('/till/tickets/3/open', { method: 'POST' })
  })

  it('someone else got there first: says so', async () => {
    api.mockImplementation(async (p: string) => { if (p === '/till/tickets') return [ticket()]; throw new Error('already opened') })
    const onOpen = vi.fn()
    render(<Strip live onOpen={onOpen} />)
    fireEvent.click(await screen.findByRole('button', { name: /Ticket 42/ }))
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'already opened')
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('before closing, each ticket can be opened, passed on or sent back', async () => {
    const t = ticket()
    api.mockImplementation(async (p: string) => (p === '/till/cashiers' ? [{ id: 8, name: 'Sara' }, { id: 7, name: 'Nimal' }] : t))
    const onOpen = vi.fn()
    const onChanged = vi.fn()
    render(<TicketsBeforeClose tickets={[t]} sym="Rs." cart={initialState('retail')} onOpen={onOpen} onChanged={onChanged} onClose={() => {}} />)
    fireEvent.click(await screen.findByRole('button', { name: 'Pass to Sara' }))
    await waitFor(() => expect(api).toHaveBeenCalledWith('/till/tickets/3/pass', { method: 'POST', json: { cashierId: 8 } }))
    expect(screen.queryByRole('button', { name: 'Pass to Nimal' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Back to Kamal' }))
    await waitFor(() => expect(api).toHaveBeenCalledWith('/till/tickets/3/back', { method: 'POST' }))
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(2))
    fireEvent.click(screen.getByRole('button', { name: 'Open and sell' }))
    await waitFor(() => expect(onOpen).toHaveBeenCalled())
  })
})
