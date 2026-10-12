import { describe, expect, it } from 'vitest'
import { serviceLine } from '@/features/till/cart/lines'
import { customLine, meteredLine, offersFor, settle, sizes, soldJobs, startPick } from './till'
import type { TillService } from './types'

const band = (price: string) => [{ minQty: 1, price }]
const svc: TillService = {
  id: 7, productId: 70, name: 'Photocopy', kind: 'metered', category: 'copy',
  offers: [
    { size: '', color: false, double: false, bands: band('10') },
    { size: 'A4', color: false, double: false, bands: band('8') },
    { size: 'A4', color: true, double: false, bands: band('40') },
    { size: 'A3', color: false, double: true, bands: band('30') },
  ],
}

describe('job panel options', () => {
  it('lists named sizes, falling back to any-size cells', () => {
    expect(sizes(svc)).toEqual(['A4', 'A3'])
    expect(offersFor(svc, 'A3').map((o) => `${o.color}${o.double}`)).toEqual(['falsetrue', 'falsefalse'])
    expect(offersFor(svc, 'A4')).toHaveLength(2) // its own B&W 1-side wins over the any-size one
  })
  it('settles a pick onto something on offer', () => {
    expect(settle(svc, { size: 'A3', color: true, double: false })).toEqual({ size: 'A3', color: false, double: false }) // keeps the side
    expect(startPick(svc)).toEqual({ size: 'A4', color: false, double: false }) // cheapest size first
    expect(settle(svc, { size: 'B5', color: false, double: false })).toEqual({ size: 'A4', color: false, double: false })
  })
})

describe('job lines', () => {
  const q = { unitPrice: '12.00', total: '252.00', minQty: 1, description: 'Photocopy A4 · B&W · 2-side', sheets: 11, cost: '22', components: [{ productId: 5, quantity: '11' }] }
  it('a metered job is a fixed-qty line carrying its paper', () => {
    const l = serviceLine(1, meteredLine(svc, { size: 'A4', color: false, double: true }, 21, q))
    expect(l).toMatchObject({ qty: 21, unitPrice: 12, fixedQty: true, description: q.description, components: q.components })
    expect(soldJobs([l])).toEqual([{ serviceId: 7, description: q.description, size: 'A4', color: false, double: true, sheets: 11 }])
  })
  it('a custom job puts the note on the line', () => {
    const l = serviceLine(2, customLine(svc, 1500, ' Nimal '))
    expect(l).toMatchObject({ qty: 1, unitPrice: 1500, description: 'Photocopy — Nimal' })
    expect(l.fixedQty).toBeUndefined()
    expect(soldJobs([l])[0]).toMatchObject({ serviceId: 7, size: null, sheets: null })
  })
})
