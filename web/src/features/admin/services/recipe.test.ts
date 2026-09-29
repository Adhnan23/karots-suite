import { describe, expect, it } from 'vitest'
import type { RecipeComponent } from '@/api/types.gen'
import { recipeCost, usedPerSale } from './recipe'

const c = (amount: string, mode: string, whole = false, costPrice = '0'): RecipeComponent => ({ productId: 1, name: '', unitAbbr: '', costPrice, amount, mode, whole })

describe('recipe cost', () => {
  it('per unit, yield and whole units', () => {
    expect(usedPerSale(c('0.018', 'per_unit'))).toBeCloseTo(0.018)
    expect(usedPerSale(c('50', 'yield'))).toBeCloseTo(0.02)
    expect(usedPerSale(c('50', 'yield', true))).toBe(1)
  })
  it('adds stock and other costs', () => {
    expect(recipeCost([c('0.2', 'per_unit', false, '50'), c('50', 'yield', false, '50')], [{ label: 'Gas', amount: '5' }])).toEqual({ stock: 11, other: 5, total: 16 })
  })
})
