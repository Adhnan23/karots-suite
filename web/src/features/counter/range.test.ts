import { afterEach, describe, expect, it, vi } from 'vitest'
import { rangeQS } from './range'

describe('list period → query string', () => {
  afterEach(() => vi.useRealTimers())
  it('presets pass through; latest sends nothing', () => {
    expect(rangeQS({ preset: '', from: '', to: '' })).toBe('')
    expect(rangeQS({ preset: 'last-week', from: '', to: '' })).toBe('&preset=last-week')
  })
  it('yesterday is an explicit one-day range (not a server preset), across a month edge', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 9, 1, 9, 0)) // 1 Oct 2026
    expect(rangeQS({ preset: 'yesterday', from: '', to: '' })).toBe('&from=2026-09-30&to=2026-09-30')
  })
  it('custom dates; a lone "from" means that one day', () => {
    expect(rangeQS({ preset: 'custom', from: '2026-09-01', to: '2026-09-15' })).toBe('&from=2026-09-01&to=2026-09-15')
    expect(rangeQS({ preset: 'custom', from: '2026-09-01', to: '' })).toBe('&from=2026-09-01&to=2026-09-01')
    expect(rangeQS({ preset: 'custom', from: '', to: '' })).toBe('')
  })
})
