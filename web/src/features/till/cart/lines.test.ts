import { describe, expect, it } from 'vitest'
import type { Lot, TillProduct } from '@/api/types.gen'
import { addProduct, lotExpired, lotsFor, serviceLine, setQty, swapProduct, unitPriceFor } from './lines'

const prod = (p: Partial<TillProduct> = {}): TillProduct => ({
  id: 7, name: 'Soap', nameLocal: '', barcode: '', sellingPrice: '100.00', wholesalePrice: '90.00', taxRate: '0',
  stockQty: '10', unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial: false,
  warrantyMonths: 0, lowStock: false, emoji: '', badges: [], packs: [], ...p,
})
const lot = (batchId: number, price: string, qty: string, expiryDate = ''): Lot => ({
  batchId, batchNo: '', expiryDate, qtyRemaining: qty, price, receivedAt: '2026-09-01',
})

describe('lots', () => {
  it('expired means strictly before today (best-before today still sells)', () => {
    expect(lotExpired(lot(1, '1', '1', '2026-09-27'), '2026-09-28')).toBe(true)
    expect(lotExpired(lot(1, '1', '1', '2026-09-28'), '2026-09-28')).toBe(false)
    expect(lotExpired(lot(1, '1', '1'), '2026-09-28')).toBe(false)
  })
  it('prompts on price disagreement or an expired FEFO-first lot only', () => {
    const today = '2026-09-28'
    expect(lotsFor({ 7: [lot(1, '100', '5'), lot(2, '120', '5')] }, 7, today)).toHaveLength(2)
    expect(lotsFor({ 7: [lot(1, '100', '5', '2026-09-01'), lot(2, '100', '5')] }, 7, today)).toHaveLength(2)
    expect(lotsFor({ 7: [lot(1, '100', '5'), lot(2, '100', '5')] }, 7, today)).toEqual([])
    expect(lotsFor(undefined, 7, today)).toEqual([])
  })
})

describe('adding', () => {
  it('wholesale uses the wholesale price when set', () => {
    expect(unitPriceFor(prod(), 'wholesale')).toBe(90)
    expect(unitPriceFor(prod({ wholesalePrice: '0.00' }), 'wholesale')).toBe(100)
  })
  it('merges the same product + lot, keeps two lots as two lines', () => {
    let r = addProduct([], 0, prod(), 'retail', undefined, [])
    r = addProduct(r.lines, r.seq, prod(), 'retail', undefined, [])
    expect(r.lines).toHaveLength(1)
    expect(r.lines[0].qty).toBe(2)
    r = addProduct(r.lines, r.seq, prod(), 'retail', lot(9, '120', '5'), [])
    expect(r.lines).toHaveLength(2)
    expect(r.lines[1]).toMatchObject({ unitPrice: 120, batchId: 9, batchLabel: 'in 2026-09-01' })
  })
  it('service products never trip found-at-till', () => {
    const r = addProduct([], 0, prod({ isService: true, stockQty: '0' }), 'retail', undefined, [])
    expect(r.lines[0].stock).toBeGreaterThan(1e9)
  })
  it('service lines are one per call, priced by the plugin', () => {
    const a = serviceLine(1, { productId: 3, name: 'Copy', price: 100, qty: 20, unitPrice: 5 })
    expect(a).toMatchObject({ qty: 20, unitPrice: 5, priceOverride: '5', isService: true })
  })
  it('a fixed-qty job line keeps its qty whatever is asked', () => {
    const a = serviceLine(1, { productId: 3, name: 'Copy', price: 100, qty: 20, unitPrice: 5, fixedQty: true })
    expect(setQty([a], 1, 1, 25, []).lines[0].qty).toBe(20)
    expect(setQty([a], 1, 1, 0, []).lines[0].qty).toBe(20)
    expect(serviceLine(2, { productId: 3, name: 'Copy', price: 5 }).fixedQty).toBeUndefined()
  })
})

describe('quantities', () => {
  it('whole-unit products round; serial boxes follow the qty', () => {
    const r0 = addProduct([], 0, prod({ trackSerial: true }), 'retail', undefined, [])
    const r = setQty(r0.lines, r0.seq, r0.key, 2.6, [])
    expect(r.lines[0].qty).toBe(3)
    expect(r.lines[0].serials).toEqual(['', '', ''])
  })
  it('a picked lot too small spills the rest onto the next lot at its price', () => {
    const lots = [lot(1, '100', '4'), lot(2, '120', '10')]
    const r0 = addProduct([], 0, prod(), 'retail', lots[0], lots)
    const r = setQty(r0.lines, r0.seq, r0.key, 10, lots)
    expect(r.lines.map((l) => [l.qty, l.unitPrice, l.batchId])).toEqual([[4, 100, 1], [6, 120, 2]])
    expect(r.notice?.tone).toBe('success')
  })
  it('no lot left to spill onto: clamp and warn', () => {
    const lots = [lot(1, '100', '4'), lot(2, '120', '1')]
    const r0 = addProduct([], 0, prod(), 'retail', lots[0], lots)
    const r = setQty(r0.lines, r0.seq, r0.key, 10, lots)
    expect(r.lines.map((l) => l.qty)).toEqual([4, 1])
    expect(r.notice?.tone).toBe('error')
  })
})

describe('swapping a line', () => {
  const short = () => {
    let r = addProduct([], 0, prod({ stockQty: '0' }), 'retail', lot(3, '100', '0'), [])
    r = setQty(r.lines, r.seq, r.key, 3, [])
    return { ...r, lines: r.lines.map((l) => ({ ...l, discount: '10', allowOversell: true })) }
  }
  it('replaces the product in place, keeps the qty, resets discount, lot and found-at-till', () => {
    const r = short()
    const out = swapProduct(r.lines, r.lines[0].key, prod({ id: 8, name: 'Lux', sellingPrice: '120.00', wholesalePrice: '0', stockQty: '9' }), 'wholesale')
    expect(out).toHaveLength(1)
    expect(out[0]).toMatchObject({ key: r.lines[0].key, productId: 8, name: 'Lux', qty: 3, unitPrice: 120, stock: 9, discount: '', batchId: 0, allowOversell: false })
  })
  it('merges into a plain line of that product already in the cart', () => {
    const r = short()
    const lux = prod({ id: 8, name: 'Lux', stockQty: '9' })
    const both = addProduct(r.lines, r.seq, lux, 'retail', undefined, [])
    const out = swapProduct(both.lines, r.lines[0].key, lux, 'retail')
    expect(out).toHaveLength(1)
    expect(out[0]).toMatchObject({ productId: 8, qty: 4 })
  })
  it('takes the asked qty when the unit differs (3 bags → 2.5 kg loose)', () => {
    const r = short()
    const loose = prod({ id: 9, name: 'Rice (loose)', unitAbbr: 'kg', unitAllowDecimal: true, stockQty: '25' })
    expect(swapProduct(r.lines, r.lines[0].key, loose, 'retail', 2.5)[0]).toMatchObject({ productId: 9, qty: 2.5 })
    // merging into a line of that product adds the asked qty, not the old line's
    const both = addProduct(r.lines, r.seq, loose, 'retail', undefined, [])
    expect(swapProduct(both.lines, r.lines[0].key, loose, 'retail', 2.5)[0]).toMatchObject({ productId: 9, qty: 3.5 })
  })
  it('never touches a group or service line', () => {
    const r = short()
    const grouped = r.lines.map((l) => ({ ...l, group: { id: 1, label: 'Deal', per: 1, count: 3 } }))
    expect(swapProduct(grouped, grouped[0].key, prod({ id: 8 }), 'retail')).toBe(grouped)
    const svc = [serviceLine(1, { productId: 5, name: 'Fix', price: 50 })]
    expect(swapProduct(svc, 1, prod({ id: 8 }), 'retail')).toBe(svc)
  })
})
