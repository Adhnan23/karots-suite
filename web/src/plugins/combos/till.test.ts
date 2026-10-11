import { describe, expect, it } from 'vitest'
import type { CartLine } from '@/features/till/cart/types'
import { soldRows } from './till'

let key = 0
const line = (unitPrice: number, qty: number, discount: string, group?: CartLine['group'], comboId?: number): CartLine => ({ key: ++key, productId: key, name: 'x', unitPrice,
  taxRate: 0, qty, stock: 9, isService: false, allowDecimal: false, batchId: 0, batchRemaining: 0, batchLabel: '', allowOversell: false, discount, discountType: 'fixed',
  trackSerial: false, warrantyMonths: 0, serials: [], priceOverride: '', description: '', components: [], group, plugin: comboId ? { combos: { comboId } } : undefined })

describe('soldRows', () => {
  it('sums each combo across its groups and ignores plain lines', () => {
    const g1 = { id: 1, label: 'A', per: 1, count: 2 }
    const g2 = { id: 5, label: 'A', per: 1, count: 1 }
    const rows = soldRows([
      line(380, 2, '25.34', g1, 7), line(220, 2, '14.66', g1, 7),
      line(380, 1, '25.34', g2, 7), line(220, 1, '14.66', g2, 7),
      line(100, 1, ''),
    ])
    expect(rows).toEqual([{ comboId: 7, sets: '3', net: '1680.00', discount: '120.00' }])
  })
})
