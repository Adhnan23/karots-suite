import { describe, expect, it } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import { changeNotes } from './change'

const copy = (d: [string, boolean][]) => ({ denominations: d.map(([value, isNote]) => ({ value, isNote })) }) as unknown as TillCopy

describe('change by note', () => {
  it("breaks the change into the shop's notes, largest first", () => {
    expect(changeNotes(1650, copy([['1000', true], ['500', true], ['100', true], ['50', true]]))).toEqual([
      { value: 1000, qty: 1, isNote: true }, { value: 500, qty: 1, isNote: true }, { value: 100, qty: 1, isNote: true }, { value: 50, qty: 1, isNote: true },
    ])
  })

  it('gives notes before coins', () => {
    expect(changeNotes(27, copy([['10', true], ['20', false], ['5', false], ['2', false]]))).toEqual([
      { value: 10, qty: 2, isNote: true }, { value: 5, qty: 1, isNote: false }, { value: 2, qty: 1, isNote: false },
    ])
  })

  it("uses Sri Lanka's notes when the shop has none listed", () => {
    expect(changeNotes(70, copy([]))).toEqual([{ value: 50, qty: 1, isNote: true }, { value: 20, qty: 1, isNote: true }])
  })
})
