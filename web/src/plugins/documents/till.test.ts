import { describe, expect, it } from 'vitest'
import { serviceLine } from '@/features/till/cart/lines'
import { customLine, materials, meteredLine, offersFor, settle, sizes, soldJobs, startPick, varies } from './till'
import type { TillService } from './types'

const band = (price: string) => [{ minQty: 1, price }]
const svc: TillService = {
  id: 7, productId: 70, name: 'Photocopy', kind: 'metered', category: 'copy',
  offers: [
    { size: '', material: '', color: false, double: false, bands: band('10') },
    { size: 'A4', material: '', color: false, double: false, bands: band('8') },
    { size: 'A4', material: '', color: true, double: false, bands: band('40') },
    { size: 'A3', material: '', color: false, double: true, bands: band('30') },
  ],
}

describe('job panel options', () => {
  it('lists named sizes, falling back to any-size cells', () => {
    expect(sizes(svc)).toEqual(['A4', 'A3'])
    expect(offersFor(svc, 'A3').map((o) => `${o.color}${o.double}`)).toEqual(['falsetrue', 'falsefalse'])
    expect(offersFor(svc, 'A4')).toHaveLength(2) // its own B&W 1-side wins over the any-size one
  })
  it('settles a pick onto something on offer', () => {
    expect(settle(svc, { size: 'A3', material: '', color: true, double: false })).toEqual({ size: 'A3', material: '', color: false, double: false }) // keeps the side
    expect(startPick(svc)).toEqual({ size: 'A4', material: '', color: false, double: false }) // cheapest size first
    expect(settle(svc, { size: 'B5', material: '', color: false, double: false })).toEqual({ size: 'A4', material: '', color: false, double: false })
  })
})

describe('job lines', () => {
  const q = { unitPrice: '12.00', total: '252.00', minQty: 1, description: 'Photocopy A4 · B&W · 2-side', sheets: 11, cost: '22', components: [{ productId: 5, quantity: '11' }] }
  it('a metered job is a fixed-qty line carrying its paper', () => {
    const l = serviceLine(1, meteredLine(svc, { size: 'A4', material: '', color: false, double: true }, 21, q))
    expect(l).toMatchObject({ qty: 21, unitPrice: 12, fixedQty: true, description: q.description, components: q.components })
    expect(soldJobs([l])).toEqual([{ serviceId: 7, description: q.description, size: 'A4', material: '', color: false, double: true, sheets: 11 }])
  })
  it('a custom job puts the note on the line', () => {
    const l = serviceLine(2, customLine(svc, 1500, ' Nimal '))
    expect(l).toMatchObject({ qty: 1, unitPrice: 1500, description: 'Photocopy — Nimal' })
    expect(l.fixedQty).toBeUndefined()
    expect(soldJobs([l])[0]).toMatchObject({ serviceId: 7, size: null, sheets: null })
  })
})

describe('types (same size, other material)', () => {
  const lam: TillService = {
    id: 9, productId: 90, name: 'Laminate', kind: 'metered', category: 'laminate',
    offers: [
      { size: 'A4', material: 'Gloss', color: false, double: false, bands: band('100') },
      { size: 'A4', material: 'Matte', color: false, double: false, bands: band('120') },
      { size: 'ID', material: '', color: false, double: false, bands: band('50') },
    ],
  }
  it('offers each size its own types and keeps a type it can', () => {
    expect(materials(lam, 'A4')).toEqual(['Gloss', 'Matte'])
    expect(materials(lam, 'ID')).toEqual([''])
    expect(settle(lam, { size: 'A4', material: 'Matte', color: false, double: false }).material).toBe('Matte')
    expect(settle(lam, { size: 'ID', material: 'Matte', color: false, double: false }).material).toBe('') // ID has no types
    expect(startPick(lam)).toEqual({ size: 'ID', material: '', color: false, double: false }) // cheapest size first
  })
  it('hides ink and sides when nothing varies', () => {
    expect(varies(lam, 'color')).toBe(false)
    expect(varies(svc, 'color')).toBe(true)
    expect(varies(svc, 'double')).toBe(true)
  })
  it('a laminate job carries its type, not ink or sides', () => {
    const q = { unitPrice: '120.00', total: '360.00', minQty: 1, description: 'Laminate A4 · Matte', sheets: 3, cost: '60', components: [{ productId: 31, quantity: '3' }] }
    const l = serviceLine(3, meteredLine(lam, { size: 'A4', material: 'Matte', color: false, double: false }, 3, q))
    expect(soldJobs([l])).toEqual([{ serviceId: 9, description: q.description, size: 'A4', material: 'Matte', color: null, double: null, sheets: 3 }])
  })
})
