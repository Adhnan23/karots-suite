import { describe, expect, it } from 'vitest'
import { lastClosedText } from './lastClosed'

describe('lastClosedText (DRAW-20)', () => {
  const now = new Date(2026, 10, 26, 10, 0)
  it('says nothing when no till was ever closed', () => {
    expect(lastClosedText(undefined, now)).toBeNull()
    expect(lastClosedText('', now)).toBeNull()
  })
  it('today, yesterday, then the date', () => {
    expect(lastClosedText(new Date(2026, 10, 26, 8, 5).toISOString(), now)).toBe('Last closed 8:05 am')
    expect(lastClosedText(new Date(2026, 10, 25, 21, 42).toISOString(), now)).toBe('Last closed yesterday 9:42 pm')
    expect(lastClosedText(new Date(2026, 10, 20, 21, 42).toISOString(), now)).toBe('Last closed 20 Nov, 9:42 pm')
  })
})
