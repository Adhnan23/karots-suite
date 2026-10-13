import { describe, expect, it } from 'vitest'
import { yieldNote } from './yield'

describe('yieldNote', () => {
  it('says nothing when the yield is as expected or blank', () => {
    expect(yieldNote(1, 25, 25, 'kg')).toBe('')
    expect(yieldNote(1, 25, 0, 'kg')).toBe('')
  })
  it('says how much less and how much dearer', () => {
    // 2500 / 24.6 − 2500 / 25 = 1.63
    expect(yieldNote(1, 25, 24.6, 'kg', { sym: 'Rs', perPack: 2500 })).toBe('0.4 kg less than 25 — cost per kg Rs 1.63 higher')
  })
  it('says more and cheaper, and leaves cost out when none is given', () => {
    expect(yieldNote(2, 50, 50.5, 'kg')).toBe('0.5 kg more than 50')
    expect(yieldNote(2, 50, 50.5, 'kg', { sym: 'Rs', perPack: 2500 })).toBe('0.5 kg more than 50 — cost per kg Rs 0.99 lower')
  })
})
