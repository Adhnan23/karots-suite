import { beforeEach, describe, expect, it } from 'vitest'
import type { Denomination, Locker } from '@/api/types.gen'
import { clearOpenDraft, countTotal, fromBreakdown, loadOpenDraft, lockerDefault, saveOpenDraft, toBreakdown } from './count'

const ds: Denomination[] = [{ value: '500', isNote: true }, { value: '100', isNote: true }, { value: '5', isNote: false }]
const lk = (id: number, kind = 'safe'): Locker => ({ id, name: `L${id}`, kind, balance: '0.00' })

describe('drawer counts', () => {
  it('totals value × count and ignores junk', () => {
    expect(countTotal(ds, { '500': '2', '100': '3', '5': 'x' })).toBe(1300)
  })
  it('breakdown keeps positive whole counts only; round-trips from the last close', () => {
    expect(toBreakdown(ds, { '500': '2', '100': '0', '5': '1.7' })).toEqual({ '500': 2, '5': 1 })
    expect(fromBreakdown({ '500': 2 })).toEqual({ '500': '2' })
    expect(fromBreakdown(undefined)).toEqual({})
  })
  it('locker default: shop default if reachable; cash-out without untracked falls back to the first', () => {
    expect(lockerDefault([lk(1), lk(2)], 2, 'in', true)).toBe('2')
    expect(lockerDefault([lk(1)], 9, 'in', true)).toBe('')
    expect(lockerDefault([lk(1)], 0, 'out', false)).toBe('1')
    expect(lockerDefault([], 0, 'out', false)).toBe('')
  })
})

describe('opening-count draft', () => {
  beforeEach(() => localStorage.clear())
  it('recovers a recent draft, drops a stale one', () => {
    saveOpenDraft({ '500': '2' }, '3')
    expect(loadOpenDraft()).toEqual({ counts: { '500': '2' }, lockerId: '3' })
    expect(loadOpenDraft(Date.now() + 13 * 3600e3)).toBeNull()
    clearOpenDraft()
    expect(loadOpenDraft()).toBeNull()
  })
})
