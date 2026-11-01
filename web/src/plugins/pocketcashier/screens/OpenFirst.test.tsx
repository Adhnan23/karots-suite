import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import { OpenFirst } from './OpenFirst'

const sendNow = vi.fn(async (_a: unknown) => ({ ref: 'r', status: 'applied' }))
vi.mock('../sync', () => ({ sendNow: (a: unknown) => sendNow(a) }))

const copy = {
  symbol: 'Rs.', places: [], defaultLockerId: 0, allowUntrackedCash: true, allowContinueLast: false,
  people: [{ id: 7, name: 'Nimal' }], denominations: [{ value: '500', isNote: true }],
} as unknown as TillCopy

describe('selling needs an open drawer, as on the PC till', () => {
  afterEach(() => { cleanup(); sendNow.mockClear() })

  it('asks to count the float, then opens the drawer for this person', async () => {
    const onSent = vi.fn()
    render(<OpenFirst who={{ id: 7, name: 'Nimal' } as never} copy={copy} onSent={onSent} />)
    expect(screen.getByText(/Open the drawer to start selling/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Count the float' }))
    fireEvent.change(screen.getByLabelText('Count of Rs. 500.00'), { target: { value: '4' } })
    fireEvent.click(screen.getByRole('button', { name: 'Open' }))
    await waitFor(() => expect(sendNow).toHaveBeenCalledWith({ kind: 'open', userId: 7, body: { openingCash: '2000', breakdown: { '500': 4 }, sourceLockerId: 0 } }))
    expect(onSent).toHaveBeenCalled()
  })
})
