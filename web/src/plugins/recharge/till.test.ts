import { describe, expect, it, vi } from 'vitest'
import type { CartLine, PayoutInput, ServiceLineInput } from '@/features/till/cart/types'
import type { TillApi } from '../slots'
import { addBill, addCashout, addPackage, addReload, chargeFor, goesBelow, inCart, tagOf } from './till'
import type { Carrier, Float, TillData } from './types'

const carrier: Carrier = { id: 1, name: 'Dialog', productId: 50, isActive: true, supplierId: 0, rates: [{ kind: 'billpay', percent: '0', fixed: '0', charge: '30' }] }
const f: Float = { id: 7, carrierId: 1, carrier: 'Dialog', label: 'Shop phone', number: '', reload: true, money: true, isActive: true, target: '0', supplierId: 0, balance: '100', lastCount: null, lastMove: null }
const data = { products: { billpay: 60, deposit: 61, cashout: 62, charge: 63 } } as TillData
function fake() {
  const lines: (ServiceLineInput | (PayoutInput & { payout: true }))[] = []
  const till = { addServiceLine: vi.fn((l: ServiceLineInput) => lines.push(l)), addPayout: vi.fn((l: PayoutInput) => lines.push({ ...l, payout: true })) } as unknown as TillApi
  return { till, lines }
}

describe('recharge cart lines', () => {
  it('a reload and a package ring on the carrier product with what the sale hook needs', () => {
    const { till, lines } = fake()
    addReload(till, carrier, f, 100, ' 0771112223 ')
    addPackage(till, carrier, f, { id: 3, carrierId: 1, name: 'Data 100', price: '100', floatCost: '88', isActive: true, sort: 0, addedBy: '' }, '')
    expect(lines[0]).toMatchObject({ productId: 50, price: 100, fixedQty: true, description: 'Dialog reload · 0771112223',
      plugin: { recharge: { kind: 'reload', device: 7, number: '0771112223' } } })
    expect(lines[1]).toMatchObject({ productId: 50, name: 'Data 100', price: 100, plugin: { recharge: { kind: 'package', device: 7, package: 3 } } })
  })
  it('a bill from a bank adds its service charge line; a cash out is a payout', () => {
    const { till, lines } = fake()
    addBill(till, data, { bank: { id: 4, name: 'BOC' } }, 2500, 'Electricity', '0123456', 30)
    addCashout(till, data, { device: f }, 500, 0)
    expect(lines).toHaveLength(3)
    expect(lines[0]).toMatchObject({ productId: 60, description: 'Electricity · 0123456', plugin: { recharge: { kind: 'billpay', bank: 4, ref: '0123456' } } })
    expect(lines[1]).toMatchObject({ productId: 63, price: 30, plugin: { recharge: { kind: 'charge' } } })
    expect(lines[2]).toMatchObject({ productId: 62, amount: 500, payout: true, plugin: { recharge: { kind: 'cashout', device: 7 } } })
    expect(chargeFor(carrier, 'billpay')).toBe('30')
    expect(chargeFor(carrier, 'reload')).toBe('')
  })
  it('the float the cart already uses, and when it would go below zero (a word, never a block)', () => {
    const l = (p: Partial<CartLine>) => ({ unitPrice: 0, qty: 1, priceOverride: '', ...p }) as CartLine
    const lines = [l({ unitPrice: 60, plugin: { recharge: { kind: 'reload', device: 7 } } }), l({ priceOverride: '20', unitPrice: -20, plugin: { recharge: { kind: 'cashout', device: 7 } } }),
      l({ unitPrice: 99, plugin: { recharge: { kind: 'reload', device: 8 } } }), l({ unitPrice: 5, plugin: { x: 1 } })]
    expect(inCart(lines, 7)).toBe(40)
    expect(goesBelow(f, 40 + 70)).toBe(true)
    expect(goesBelow(f, 100)).toBe(false)
    expect(tagOf({ x: 1 })).toBeNull()
  })
})
