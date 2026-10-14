import { describe, expect, it } from 'vitest'
import { dueBadge, qtyOf, waNumber } from './labels'

const now = new Date('2026-10-14T10:00:00+05:30')
const at = (d: string) => new Date(`${d}T09:00:00+05:30`).toISOString()

describe('dueBadge', () => {
  it('names the day it runs out', () => {
    expect(dueBadge('overdue', at('2026-10-11'), now)).toEqual({ text: 'Ran out 3 days ago', tone: 'danger' })
    expect(dueBadge('overdue', at('2026-10-13'), now)).toEqual({ text: 'Ran out yesterday', tone: 'danger' })
    expect(dueBadge('soon', at('2026-10-14'), now)).toEqual({ text: 'Runs out today', tone: 'warn' })
    expect(dueBadge('soon', at('2026-10-15'), now)).toEqual({ text: 'Runs out tomorrow', tone: 'warn' })
    expect(dueBadge('soon', at('2026-10-17'), now)).toEqual({ text: 'Runs out in 3 days', tone: 'warn' })
    expect(dueBadge('covered', at('2026-11-02'), now)).toEqual({ text: 'Covered till 2 Nov', tone: 'muted' })
  })
  it('none before the first hand-over', () => expect(dueBadge('', null, now)).toBeNull())
})

describe('qtyOf', () => {
  it('what Add puts in: the owed tablets, else the prescribed qty, else 1', () => {
    expect(qtyOf({ owed: '38', quantity: '60' })).toBe(38)
    expect(qtyOf({ owed: '0', quantity: '60' })).toBe(60)
    expect(qtyOf({ owed: '0', quantity: '0' })).toBe(1)
  })
})

describe('waNumber', () => {
  it('local 0… becomes 94…', () => {
    expect(waNumber('077 123-4567')).toBe('94771234567')
    expect(waNumber('+94771234567')).toBe('94771234567')
  })
})
