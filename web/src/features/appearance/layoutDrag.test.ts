import { describe, expect, it } from 'vitest'
import { dragBar, innerOf, minsOf, startShares } from './layoutDrag'

describe('dragBar', () => {
  const inner = 1000
  it('moves the share between the two columns beside the bar', () => {
    const fr = dragBar([0.4, 0.35, 0.25], 1, 50, inner, [300, 340, 330].map((m) => m / 2))
    expect(fr[0]).toBeCloseTo(0.45)
    expect(fr[1]).toBeCloseTo(0.3)
    expect(fr[2]).toBe(0.25)
  })
  it('stops at a minimum and the shares still add up to one', () => {
    const fr = dragBar([0.4, 0.35, 0.25], 1, 500, inner, [300, 340, 330])
    expect(fr[1] * inner).toBeCloseTo(340)
    expect(fr.reduce((a, b) => a + b)).toBeCloseTo(1)
  })
})

describe('startShares', () => {
  it('a three-column arrangement starts from today’s widths, by section', () => {
    const fr = startShares('CPY', 1366)!
    expect(fr[0] * innerOf('CPY', 1366)).toBeCloseTo(392) // the cart, now on the left
    expect(fr.reduce((a, b) => a + b)).toBeCloseTo(1)
  })
  it('every column meets its minimum, or the arrangement does not fit (null)', () => {
    const fr = startShares('P_CY', 1280)!
    const mins = minsOf('P_CY')
    fr.forEach((f, i) => expect(f * innerOf('P_CY', 1280)).toBeGreaterThanOrEqual(mins[i] - 0.01))
    expect(startShares('PCY', 900)).toBeNull() // 300 + 340 + 330 + gaps > 900
  })
})
