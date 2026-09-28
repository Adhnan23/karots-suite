import { describe, expect, it } from 'vitest'
import type { Lot, TillProduct } from '@/api/types.gen'
import { addProduct, lotExpired, lotsFor, serviceLine, setQty, unitPriceFor } from './lines'

const prod = (p: Partial<TillProduct> = {}): TillProduct => ({
  id: 7, name: 'Soap', barcode: '', sellingPrice: '100.00', wholesalePrice: '90.00', taxRate: '0',
  stockQty: '10', unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial: false,
  warrantyMonths: 0, emoji: '', badges: [], ...p,
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
