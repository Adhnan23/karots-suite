import { describe, expect, it } from 'vitest'
import { shortForChange } from './totals'

describe('shortForChange', () => {
  it('is what the drawer lacks for the change (the customer’s own notes can’t be broken into it)', () => {
    expect(shortForChange(50, 0)).toBe(50) // emptied into the safe at close
    expect(shortForChange(50, 30)).toBe(20)
    expect(shortForChange(50, 50)).toBe(0)
    expect(shortForChange(0, 0)).toBe(0) // exact cash
    expect(shortForChange(50, -10)).toBe(50) // never more than the change itself
  })
})
