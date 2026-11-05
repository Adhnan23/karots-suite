import { describe, expect, it } from 'vitest'
import type { CartLine } from '@/features/till/cart/types'
import { groupsCsv, hasTiers, isShort, staysAuto, tierStatus } from './labels'
import type { Group, Tier } from './types'

const tier = (p: Partial<Tier> = {}): Tier => ({ id: 1, name: '', reorderLevel: 0, levelAuto: false, splitAuto: true, active: true, total: '0', low: false, members: [], ...p })
const group = (tiers: Tier[]): Group => ({ id: 1, name: 'USB "32GB"', active: true, products: 0, tiers })

describe('alternatives labels', () => {
  it('renaming an Auto tier keeps it Auto; typing a new level makes it typed', () => {
    const t = tier({ levelAuto: true, reorderLevel: 30 })
    expect(staysAuto(t, 30)).toBe(true)
    expect(staysAuto(t, 5)).toBe(false)
    expect(staysAuto(tier({ levelAuto: false, reorderLevel: 30 }), 30)).toBe(false)
  })
  it('an Auto tier with no level yet is learning, a typed 0 is not tracked', () => {
    expect(tierStatus(tier({ levelAuto: true }))).toBe('Learning')
    expect(tierStatus(tier({ levelAuto: false }))).toBe('Not tracked')
  })
  it('a group has tiers once one is named or there are two', () => {
    expect(hasTiers(group([tier()]))).toBe(false)
    expect(hasTiers(group([tier({ name: 'Genuine' })]))).toBe(true)
    expect(hasTiers(group([tier(), tier({ id: 2, name: 'x' })]))).toBe(true)
  })
  it('status: untracked, low, ok', () => {
    expect(tierStatus(tier())).toBe('Not tracked')
    expect(tierStatus(tier({ reorderLevel: 5, low: true }))).toBe('Low')
    expect(tierStatus(tier({ reorderLevel: 5 }))).toBe('OK')
  })
  it('a short line is plain and over its stock', () => {
    const l = { qty: 3, stock: 2, isService: false } as CartLine
    expect(isShort(l)).toBe(true)
    expect(isShort({ ...l, stock: 3 })).toBe(false)
    expect(isShort({ ...l, group: { id: 1, label: 'x', per: 1, count: 3 } })).toBe(false)
    expect(isShort({ ...l, isService: true })).toBe(false)
  })
  it('csv: a row per tier, quotes escaped', () => {
    const csv = groupsCsv([group([tier({ name: 'Genuine', total: '4.000000', reorderLevel: 5, low: true })])])
    expect(csv.split('\n')[1]).toBe('"USB ""32GB""","Genuine",4,5,Low')
  })
})
