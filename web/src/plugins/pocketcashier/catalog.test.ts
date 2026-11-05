import { describe, expect, it } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { byBarcode, search } from './catalog'

const prod = (p: Partial<CopyProduct>) => ({
  id: 1, name: 'x', nameLocal: '', barcode: '', sellingPrice: '10.00', wholesalePrice: '0.00', taxRate: '0', stockQty: '5',
  unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial: false, warrantyMonths: 0, lowStock: false, emoji: '',
  badges: [], packs: [], categoryId: 1, barcodes: [], packCodes: [], ...p,
}) as CopyProduct

const copy = {
  products: [
    prod({ id: 1, name: 'Samba Rice 25kg', nameLocal: 'සම්බා සහල්', barcode: '4790001' }),
    prod({ id: 2, name: 'Sunlight soap', barcode: '4790002', barcodes: ['111'], packs: [{ id: 9, name: 'Box', size: '24', sellingPrice: '0.00', wholesalePrice: '0.00' }], packCodes: [{ packId: 9, barcode: '4790099' }] }),
  ],
} as unknown as TillCopy

describe('the phone catalogue', () => {
  it('finds by name, any case, and by local name', () => {
    expect(search(copy, 'samba').map((p) => p.id)).toEqual([1])
    expect(search(copy, 'SUNLIGHT').map((p) => p.id)).toEqual([2])
    expect(search(copy, 'සම්බා').map((p) => p.id)).toEqual([1])
  })
  it('finds by barcode, extra code and pack code', () => {
    expect(byBarcode(copy, '4790001')?.product.id).toBe(1)
    expect(byBarcode(copy, '111')?.product.id).toBe(2)
    const pack = byBarcode(copy, '4790099')
    expect(pack?.product.id).toBe(2)
    expect(pack?.packId).toBe(9)
    expect(byBarcode(copy, '000')).toBeNull()
  })
})

describe('what the phone sells', () => {
  it('sells a plugin-claimed product through its menu, and leaves out an unclaimed unpriced service', async () => {
    const { slots } = await import('@/plugins/slots')
    const { sellable } = await import('./catalog')
    slots.tillMenu.push({ key: 'x', label: 'X', claims: (p) => p.badges.includes('🖨'), Panel: () => null })
    expect(sellable(prod({ id: 1 }))).toBe(true)
    expect(sellable(prod({ id: 2, badges: ['🖨'] }))).toBe(true)
    expect(sellable(prod({ id: 5, isService: true, sellingPrice: '0.00', badges: ['🖨'] }))).toBe(true)
    expect(sellable(prod({ id: 3, isService: true, sellingPrice: '0.00' }))).toBe(false)
    expect(sellable(prod({ id: 4, isService: true, sellingPrice: '500.00' }))).toBe(true)
    slots.tillMenu.pop()
  })
})
