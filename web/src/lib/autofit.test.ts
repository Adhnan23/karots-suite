import { describe, expect, it } from 'vitest'
import { autofitZoom } from './autofit'

describe('autofitZoom', () => {
  it('never enlarges', () => expect(autofitZoom(1000, 1920)).toBe(1))
  it('leaves a screen that already fits alone', () => expect(autofitZoom(1366, 1366)).toBe(1))
  it('shrinks to fit with 16px slack', () => expect(autofitZoom(1384, 1366)).toBeCloseTo(1366 / 1400, 4))
})
