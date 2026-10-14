import { describe, expect, it } from 'vitest'
import type { SaleLine } from '@/api/types.gen'
import { saleLineBlocks } from './saleLines'

const line = (id: number, group = 0, subtotal = '100.00'): SaleLine => ({ id, name: `L${id}`, unitAbbr: 'pc', quantity: '2', returnedQty: '0', unitPrice: '60.00',
  subtotal, refundUnit: '50.00', paid: subtotal, refundLeft: subtotal, isService: false, group, groupLabel: group ? 'Pair (combo)' : '', groupSets: group ? '2' : '', packLabel: '' })

describe('saleLineBlocks', () => {
  it('gathers a group where its first line stood and sums its net', () => {
    const b = saleLineBlocks([line(1), line(2, 1, '70.00'), line(3), line(4, 1, '30.50')])
    expect(b.map((x) => x.kind)).toEqual(['line', 'group', 'line'])
    const g = b[1]
    expect(g.kind === 'group' && [g.label, g.sets, g.net, g.lines.length]).toEqual(['Pair (combo)', '2', 100.5, 2])
  })
  it('a sale without groups is its lines', () => {
    expect(saleLineBlocks([line(1), line(2)]).every((x) => x.kind === 'line')).toBe(true)
  })
})
