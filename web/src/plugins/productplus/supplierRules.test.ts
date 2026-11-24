import { describe, expect, it } from 'vitest'
import { matchValues } from './supplierRules'

const v = (value: string, products = 1, supplierId = 0) => ({ value, products, supplierId, supplierName: '' })

describe('matchValues', () => {
  const rows = [v('Bosch', 3), v('NGK', 1, 7), v('Denso Japan', 2)]
  it('keeps the server order with no search', () => {
    expect(matchValues(rows, '  ').map((r) => r.value)).toEqual(['Bosch', 'NGK', 'Denso Japan'])
  })
  it('matches any part, any case, ignoring spaces at the ends', () => {
    expect(matchValues(rows, ' jap ').map((r) => r.value)).toEqual(['Denso Japan'])
    expect(matchValues(rows, 'BOS').map((r) => r.value)).toEqual(['Bosch'])
  })
  it('matches the supplier name too', () => {
    expect(matchValues([v('A'), { ...v('B'), supplierName: 'Lanka Auto' }], 'lanka').map((r) => r.value)).toEqual(['B'])
  })
})
