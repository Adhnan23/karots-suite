import { describe, expect, it } from 'vitest'
import type { CartLine } from '@/features/till/cart/types'
import { findHints, findPartials } from './match'
import type { Combo, Member } from './types'

const mem = (productId: number, qty = '1'): Member => ({ productId, name: `P${productId}`, qty, sellingPrice: '100.00', wholesalePrice: '0.00', active: true, isService: false, stock: '9' })
const combo = (id: number, members: Member[], problem = ''): Combo => ({ id, name: `C${id}`, discountType: 'percent', discountValue: '10', active: true, startsOn: '', endsOn: '',
  window: 'on', members, fullPrice: '0', problem, warnings: [], soldSets: '0', soldNet: '0', lastSold: null })
let key = 0
const line = (productId: number, qty: number, extra: Partial<CartLine> = {}): CartLine => ({ key: ++key, productId, name: `P${productId}`, unitPrice: 100, taxRate: 0, qty, stock: 9,
  isService: false, allowDecimal: false, batchId: 0, batchRemaining: 0, batchLabel: '', allowOversell: false, discount: '', discountType: 'fixed', trackSerial: false,
  warrantyMonths: 0, serials: [], priceOverride: '', description: '', components: [], ...extra })

describe('combo matching', () => {
  const pair = combo(1, [mem(1), mem(2)])
  const bogo = combo(2, [mem(3, '4')])
  it('counts full sets across plain lines and takes exactly those units', () => {
    const a = line(1, 2), b = line(2, 1), b2 = line(2, 2)
    const [h] = findHints([pair], [a, b, b2])
    expect(h.sets).toBe(2)
    expect(h.take).toEqual([{ key: a.key, qty: 2 }, { key: b.key, qty: 1 }, { key: b2.key, qty: 1 }])
  })
  it('skips grouped, discounted, lot-picked and service lines', () => {
    const lines = [line(1, 1, { group: { id: 9, label: 'x', per: 1, count: 1 } }), line(1, 1, { discount: '5' }), line(1, 1, { batchId: 4 }), line(1, 1, { isService: true }), line(2, 1)]
    expect(findHints([pair], lines)).toEqual([])
  })
  it('a blocked combo is never hinted', () => {
    expect(findHints([combo(3, [mem(1), mem(2)], 'P2 is disabled')], [line(1, 1), line(2, 1)])).toEqual([])
  })
  it('a part set offers what is missing; a full set is a hint, not a part', () => {
    const p = findPartials([pair, bogo], [line(1, 1), line(3, 2)])
    expect(p.map((x) => [x.combo.id, x.missing])).toEqual([[1, ['P2']], [2, ['2 × P3']]])
    expect(p[1].take[0].qty).toBe(2)
    expect(findPartials([pair], [line(1, 1), line(2, 1)])).toEqual([])
    expect(findPartials([pair], [line(5, 1)])).toEqual([])
    expect(findPartials([pair], [line(1, 1), line(2, 1, { batchId: 3 })])).toEqual([]) // P2 is there, on a picked lot
  })
})
