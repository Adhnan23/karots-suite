import { describe, expect, it } from 'vitest'
import type { CartLine } from '@/features/till/cart/types'
import { groupsCsv, hasTiers, isShort, tierStatus } from './labels'
import type { Group, Tier } from './types'

const tier = (p: Partial<Tier> = {}): Tier => ({ id: 1, name: '', reorderLevel: 0, active: true, total: '0', low: false, members: [], ...p })
const group = (tiers: Tier[]): Group => ({ id: 1, name: 'USB "32GB"', active: true, products: 0, tiers })

describe('alternatives labels', () => {
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
