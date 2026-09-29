import { describe, expect, it } from 'vitest'
import { editPrice, marginText } from './price'

describe('cost ⇄ markup ⇄ selling', () => {
  const p = { cost: '100', markup: '1.25', selling: '125.00' }
  it('cost follows through the markup', () => expect(editPrice(p, 'cost', '80').selling).toBe('100.00'))
  it('markup sets selling', () => expect(editPrice(p, 'markup', '1.5').selling).toBe('150.00'))
  it('selling sets markup', () => expect(editPrice(p, 'selling', '130').markup).toBe('1.3'))
  it('no markup yet: cost derives it from selling', () => expect(editPrice({ cost: '', markup: '', selling: '50' }, 'cost', '40').markup).toBe('1.25'))
  it('whole rounds to the rupee', () => expect(editPrice({ cost: '', markup: '1.4', selling: '' }, 'cost', '333', true).selling).toBe('466'))
  it('margin', () => expect(marginText('75', '100')).toBe('25% margin'))
})
