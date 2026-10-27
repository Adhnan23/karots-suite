import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { Fold } from './Fold'

const details = () => screen.getByText('Labels').closest('details')!

describe('Fold', () => {
  beforeEach(() => localStorage.clear())
  afterEach(cleanup)

  it('starts as told, shows its hint, and remembers being opened on this device', () => {
    const { unmount } = render(<Fold title="Labels" hint="Price Tag" remember="labels"><p>body</p></Fold>)
    expect(details().open).toBe(false)
    expect(screen.getByText('Price Tag')).toBeTruthy()
    details().open = true
    fireEvent(details(), new Event('toggle'))
    unmount()
    render(<Fold title="Labels" remember="labels"><p>body</p></Fold>)
    expect(details().open).toBe(true)
  })

  it('a remembered close wins over open-by-default; without remember it just follows `open`', () => {
    localStorage.setItem('ks-fold', JSON.stringify({ palette: false }))
    const { unmount } = render(<Fold title="Labels" open remember="palette"><p>body</p></Fold>)
    expect(details().open).toBe(false)
    unmount()
    render(<Fold title="Labels" open><p>body</p></Fold>)
    expect(details().open).toBe(true)
  })
})
