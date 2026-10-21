import { describe, expect, it } from 'vitest'
import type { ReorderCalcView } from '@/api/types.gen'
import { learning, looksOff, perDay, reasonLine } from './reason'

const calc = (c: Partial<ReorderCalcView>): ReorderCalcView => ({
  daily: 10, recentDaily: 10, lastYearDaily: 0, rhythmDays: 7, rhythmFrom: 'product', regular: true, safetyDays: 3,
  calcLevel: 100, learnedPct: 100, historyDays: 200, notSelling: false, cappedBills: 0, ...c,
})

describe('reorder reason lines', () => {
  it('says how fast it sells and how often the supplier comes', () => {
    expect(reasonLine(calc({}))).toBe('sells 10 a day · supplier every 7 days · + 3 safety days')
  })
  it('an irregular supplier is planned for its longest gap', () => {
    expect(reasonLine(calc({ regular: false, rhythmDays: 12 }))).toBe('sells 10 a day · supplier comes irregularly, planned for 12 days · + 3 safety days')
  })
  it('no delivery history uses the shop default', () => {
    expect(reasonLine(calc({ rhythmFrom: 'default' }))).toBe('sells 10 a day · no deliveries recorded, planning for 7 days · + 3 safety days')
  })
  it('last year leads when it was busier then', () => {
    expect(reasonLine(calc({ daily: 15, recentDaily: 2, lastYearDaily: 15, rhythmDays: 10 })))
      .toBe('Last year this time: 15 a day · now 2 a day · supplier every 10 days · + 3 safety days')
  })
  it('mentions ignored one-off bills', () => {
    expect(reasonLine(calc({ cappedBills: 1 }))).toContain('· 1 big bill ignored')
    expect(reasonLine(calc({ cappedBills: 2 }))).toContain('· 2 big bills ignored')
  })
  it('not selling', () => {
    expect(reasonLine(calc({ notSelling: true, daily: 0, recentDaily: 0 }))).toBe('no sales in 90 days, none this time last year')
  })
  it('per day reads naturally', () => {
    expect([perDay(10), perDay(0.6), perDay(1.25), perDay(1.5)]).toEqual(['10 a day', '0.6 a day', '1.3 a day', '1.5 a day'])
  })
  it('learning shows weeks of 8', () => {
    expect(learning(calc({ learnedPct: 37, historyDays: 21 }))).toBe('learning, 3 of 8 weeks')
    expect(learning(calc({}))).toBe('')
  })
  it('a Fixed level looks off at 2× / ½ and 3+ units apart, once learned (as the server)', () => {
    expect(looksOff(20, calc({ calcLevel: 6 }))).toBe(true)
    expect(looksOff(10, calc({ calcLevel: 15 }))).toBe(false)
    expect(looksOff(2, calc({ calcLevel: 4 }))).toBe(false)
    expect(looksOff(20, calc({ calcLevel: 6, learnedPct: 50 }))).toBe(false)
    expect(looksOff(0, calc({ calcLevel: 5 }))).toBe(true)
  })
})
