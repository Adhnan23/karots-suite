import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import { DrawerSheet } from './DrawerSheet'

const copy = {
  symbol: 'Rs.', places: [], defaultLockerId: 0, allowUntrackedCash: true,
  denominations: [{ value: '1000', isNote: true }, { value: '500', isNote: true }, { value: '10', isNote: false }],
} as unknown as TillCopy

describe('the phone drawer sheet', () => {
  afterEach(cleanup)

  it('opens with a float counted note by note, as the PC till does', async () => {
    const onSend = vi.fn(async () => '')
    render(<DrawerSheet kind="open" copy={copy} expected="0" onSend={onSend} onClose={() => {}} />)
    fireEvent.change(screen.getByLabelText('Count of Rs. 500.00'), { target: { value: '2' } })
    fireEvent.change(screen.getByLabelText('Count of Rs. 10.00'), { target: { value: '5' } })
    expect(screen.getByText('Rs. 1,050.00')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Open' }))
    await waitFor(() => expect(onSend).toHaveBeenCalled())
    expect(onSend).toHaveBeenCalledWith({ openingCash: '1050', breakdown: { '500': 2, '10': 5 }, sourceLockerId: 0 })
  })

  it('closes with the cash counted note by note', async () => {
    const onSend = vi.fn(async () => '')
    render(<DrawerSheet kind="close" copy={copy} expected="2000" onSend={onSend} onClose={() => {}} />)
    fireEvent.change(screen.getByLabelText('Count of Rs. 1,000.00'), { target: { value: '2' } })
    fireEvent.click(screen.getAllByRole('button', { name: 'Close' }).find((b) => b.dataset.slot !== 'sheet-close')!) // not the sheet's ×
    await waitFor(() => expect(onSend).toHaveBeenCalled())
    expect(onSend).toHaveBeenCalledWith({ closingCash: '2000', breakdown: { '1000': 2 }, destLockerId: 0 })
  })
})
