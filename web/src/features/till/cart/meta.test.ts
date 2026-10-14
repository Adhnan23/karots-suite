import { describe, expect, it } from 'vitest'
import type { HeldSale, TillProduct } from '@/api/types.gen'
import { fromHeld, toHeldBlob } from './held'
import { initialState, tillReducer } from './store'

const p: TillProduct = { id: 5, name: 'Tab', barcode: '', sellingPrice: '10.00', wholesalePrice: '0.00', taxRate: '0', stockQty: '50',
  unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial: false, warrantyMonths: 0, emoji: '', badges: [], packs: [] }

describe('plugin seams on the sale', () => {
  it('sale notes survive hold → resume and are cleared by New sale', () => {
    let s = tillReducer(initialState('retail'), { type: 'saleMeta', key: 'notes', value: { patientId: 3 } })
    const h: HeldSale = { id: 1, label: 'x', saleType: 'retail', discount: '0', cart: toHeldBlob(s), itemCount: 0, total: '0', createdAt: '' }
    expect(fromHeld(h, 0).meta).toEqual({ notes: { patientId: 3 } })
    s = tillReducer(s, { type: 'resume', held: h })
    expect(s.meta).toEqual({ notes: { patientId: 3 } })
    s = tillReducer(s, { type: 'saleMeta', key: 'notes', value: undefined })
    expect(s.meta).toEqual({})
    s = tillReducer(tillReducer(s, { type: 'saleMeta', key: 'k', value: 1 }), { type: 'newSale', saleType: 'retail' })
    expect(s.meta).toEqual({})
  })
  it('a plugin-tagged add stays its own line; the same tag merges', () => {
    let s = tillReducer(initialState('retail'), { type: 'add', product: p, lots: [] })
    s = tillReducer(s, { type: 'add', product: p, lots: [], units: 4, plugin: { rx: 7 } })
    s = tillReducer(s, { type: 'add', product: p, lots: [], units: 2, plugin: { rx: 7 } })
    expect(s.lines.map((l) => [l.qty, l.plugin])).toEqual([[1, undefined], [6, { rx: 7 }]])
  })
})
