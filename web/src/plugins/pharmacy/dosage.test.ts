import { describe, expect, it } from 'vitest'
import { dosesPerDay, supplyDays } from './dosage'

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
