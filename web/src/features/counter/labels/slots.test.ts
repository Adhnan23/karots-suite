import { describe, expect, it } from 'vitest'
import type { LabelRequest, TillConfig } from '@/api/types.gen'
import { toPriceLabel } from '@/features/print/labels/price'

const cfg = { labelFirstLines: 2, labelSecondLines: 1, labelFirstSize: '2', labelSecondSize: '4' } as TillConfig
const req = (r: Partial<LabelRequest>): LabelRequest => ({ productId: 1, code: '4792024012348', format: 'EAN13', top: 'Milk', bottom: 'Rs 90.00', count: 1, size: '', ...r })

describe('toPriceLabel', () => {
  it('carries which field each text is, and the shop’s lines and sizes', () => {
    const d = toPriceLabel(req({ topKey: 'price', bottomKey: 'name', top: 'Rs 90.00', bottom: 'Milk' }), cfg)
    expect(d.first).toEqual({ key: 'price', text: 'Rs 90.00' })
    expect(d.second).toEqual({ key: 'name', text: 'Milk' })
    expect([d.firstLines, d.secondLines, d.firstSize, d.secondSize]).toEqual([2, 1, '2', '4'])
    expect([d.code, d.format]).toEqual(['4792024012348', 'EAN13'])
  })
  it('typed text (a custom label, a plugin) is free; a blank text is none', () => {
    const d = toPriceLabel(req({ top: 'Model X', bottom: ' ' }), cfg)
    expect(d.first.key).toBe('free')
    expect(d.second).toEqual({ key: 'none', text: '' })
  })
  it('a slot chosen as none stays none', () => {
    expect(toPriceLabel(req({ topKey: 'none', top: '' }), cfg).first.key).toBe('none')
  })
})
