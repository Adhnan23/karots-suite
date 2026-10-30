import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { SaleView } from '@/api/types.gen'
import { ReviewBlock, returnSource } from './SaleReview'

const sale = {
  createdAt: '2026-10-30T10:30:00Z', rungAt: '2026-10-30T08:05:00Z', tillSource: 'till:7',
  flags: [
    { kind: 'oversold', note: 'Tea: sold 3, the count showed 1', createdAt: '2026-10-30T10:30:00Z' },
    { kind: 'price', note: 'Tea sold at 100.00, price here 120.00', createdAt: '2026-10-30T10:30:00Z' },
  ],
} as SaleView

describe('SaleSheet shows flags', () => {
  it('lists every flag under Needs review, with the rung time', () => {
    render(<ReviewBlock sale={sale} />)
    expect(screen.getByText('Needs review')).toBeTruthy()
    expect(screen.getByText(/the count showed 1/)).toBeTruthy()
    expect(screen.getByText(/price here 120/)).toBeTruthy()
    expect(screen.getByText(/Rung at/)).toBeTruthy()
  })
  it('shows nothing for a plain sale', () => {
    const { container } = render(<ReviewBlock sale={{ ...sale, flags: [], rungAt: undefined }} />)
    expect(container.textContent).toBe('')
  })
  it('refunds from the till that took it by default', () => {
    expect(returnSource(sale)).toBe('till:7')
  })
})
