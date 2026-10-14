import { describe, expect, it } from 'vitest'
import { dosesPerDay, perDayOf, supplyDays, tabletsFor } from './dosage'

describe('dosesPerDay (mirrors Go DosesPerDay)', () => {
  it.each([
    ['1-0-1', 2], ['1-1-1', 3], ['0-0-1', 1], ['1/2-0-1/2', 1], ['½-0-½', 1], ['2-0-2', 4], ['1-1-1-1', 4],
    [' 1 - 0 - 1 ', 2], ['BD', 2], ['tds', 3], ['od', 1], ['nocte', 1], ['qid', 4],
    ['1 tablet twice daily', 0], ['', 0], ['1-0', 0], ['a-b-c', 0],
  ])('%s → %d', (s, n) => expect(dosesPerDay(s)).toBe(n))
})

describe('supplyDays', () => {
  it('whole days the tablets last', () => {
    expect(supplyDays(60, 2)).toBe(30)
    expect(supplyDays(14, 3)).toBe(4)
    expect(supplyDays(10, 0)).toBe(0)
  })
})

describe('tabletsFor', () => {
  it('dose × days, a part tablet rounds up', () => {
    expect(tabletsFor(2, 2)).toBe(4) // 1-0-1 × 2 days
    expect(tabletsFor(1.5, 3)).toBe(5) // 1/2-0-1 × 3 = 4.5 → 5
    expect(tabletsFor(0, 3)).toBe(0)
    expect(tabletsFor(2, 0)).toBe(0)
  })
})

describe('perDayOf', () => {
  const mem = [{ dose: '1 every 8 hours', perDay: '3', uses: 4 }, { dose: '1-0-1', perDay: '2', uses: 2 }]
  it('remembered doses first, then the 1-0-1 reader', () => {
    expect(perDayOf('1 Every 8 Hours ', mem)).toBe(3)
    expect(perDayOf('1-1-1', mem)).toBe(3)
    expect(perDayOf('weekly', mem)).toBe(0)
  })
})
