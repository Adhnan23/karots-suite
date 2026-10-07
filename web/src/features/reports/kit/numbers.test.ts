import { describe, expect, it } from 'vitest'
import { delta, pctText, share } from './numbers'

describe('report numbers', () => {
  it('delta is the % change, or null with no baseline', () => {
    expect(delta('150', '100')).toBe(50)
    expect(delta('50', '100')).toBe(-50)
    expect(delta('10', '0')).toBeNull()
    expect(delta('0', '0')).toBeNull()
    expect(delta('-20', '-10')).toBe(-100) // a loss twice as deep reads as worse
  })
  it('share is part of total in %, or null when the total is not positive', () => {
    expect(share('25', '100')).toBe(25)
    expect(share('5', '0')).toBeNull()
    expect(share('5', '-10')).toBeNull()
  })
  it('pctText never prints NaN or Infinity', () => {
    expect(pctText(null)).toBe('—')
    expect(pctText(12.345)).toBe('12.3%')
    expect(pctText(-0.04)).toBe('0.0%')
  })
})
