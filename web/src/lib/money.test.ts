import { describe, expect, it } from 'vitest'
import { dec, money, num, qtyText, r2 } from './money'

describe('money', () => {
  it('formats with the shop symbol, grouping and 2 decimals', () => {
    expect(money('Rs.', 1234.5)).toBe('Rs. 1,234.50')
    expect(money('Rs.', '99')).toBe('Rs. 99.00')
    expect(money('Rs.', -50)).toBe('Rs. -50.00')
  })
  it('parses loosely and never returns NaN', () => {
    expect(num('1,200')).toBe(1200)
    expect(num('abc')).toBe(0)
    expect(num(undefined)).toBe(0)
    expect(num(' 12.5 ')).toBe(12.5)
  })
  it('rounds to cents without float drift', () => {
    expect(r2(0.1 + 0.2)).toBe(0.3)
    expect(r2(1.005)).toBe(1.01)
    expect(dec(10)).toBe('10.00')
  })
  it('prints quantities without forced decimals', () => {
    expect(qtyText(137)).toBe('137')
    expect(qtyText(1.5)).toBe('1.5')
    expect(qtyText(0.3333333)).toBe('0.333')
  })
})
