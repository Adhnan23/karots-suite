import { describe, expect, it } from 'vitest'
import type { CartLine } from '@/features/till/cart/types'
import { hasTiers, isShort, staysAuto } from './labels'
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
  it('a group has tiers once one is named or there are two', () => {
    expect(hasTiers(group([tier()]))).toBe(false)
    expect(hasTiers(group([tier({ name: 'Genuine' })]))).toBe(true)
    expect(hasTiers(group([tier(), tier({ id: 2, name: 'x' })]))).toBe(true)
  })
  it('a short line is plain and over its stock', () => {
    const l = { qty: 3, stock: 2, isService: false } as CartLine
    expect(isShort(l)).toBe(true)
    expect(isShort({ ...l, stock: 3 })).toBe(false)
    expect(isShort({ ...l, group: { id: 1, label: 'x', per: 1, count: 3 } })).toBe(false)
    expect(isShort({ ...l, isService: true })).toBe(false)
  })
})
