import { describe, expect, it } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState, tillReducer } from '@/features/till/cart/store'
import type { Payment, TillState } from '@/features/till/cart/types'
import { effectivePayments } from '@/features/till/cart/checkout'
import { saleTotal } from '@/features/till/cart/totals'
import { payIssue, saleBody } from './sale'

const rice = {
  id: 1, name: 'Rice', nameLocal: '', barcode: '', sellingPrice: '120.00', wholesalePrice: '100.00', taxRate: '0', stockQty: '50',
  unitAbbr: 'kg', unitAllowDecimal: true, isService: false, trackSerial: false, warrantyMonths: 0, lowStock: false, emoji: '',
  badges: [], packs: [{ id: 9, name: 'Bag 5kg', size: '5', sellingPrice: '550.00', wholesalePrice: '0.00' }], categoryId: 1, barcodes: [], packCodes: [],
} as unknown as CopyProduct

const copy = { symbol: 'Rs.', products: [rice], places: [{ id: 3, name: 'Card machine', kind: 'card', cashierAccess: true }], customers: [] } as unknown as TillCopy

const cart = (type: 'retail' | 'wholesale', units?: number): TillState =>
  tillReducer(initialState(type), { type: 'add', product: rice, lots: [], units })

const pay = (c: TillState, ...ps: Partial<Payment>[]): TillState =>
  ({ ...c, tenders: ps.map((p) => ({ method: 'cash', amount: '', reference: '', accountId: '', meta: {}, ...p })) })

describe('the phone sale', () => {
  it('keeps product ids and qty, sends the shelf price it showed, and never kicks the drawer', () => {
    const c = cart('retail', 5) // one Bag 5kg
    const b = saleBody(c, effectivePayments(c, saleTotal(c)))
    expect(b.items).toHaveLength(1)
    expect(b.items[0]).toMatchObject({ productId: 1, quantity: '5', expectPrice: '120' })
    expect(b.deferDrawerKick).toBe(true)
    expect(b.total).toBe('550.00') // five units ring as one Bag 5kg
    expect(b.payments[0]).toMatchObject({ method: 'cash', amount: '550' })
  })

  it('sends the wholesale price on a wholesale cart', () => {
    const c = cart('wholesale')
    expect(saleBody(c, effectivePayments(c, saleTotal(c))).items[0].expectPrice).toBe('100')
  })

  it('takes an untouched cash draft on a walk-in as exact cash, as the till does', () => {
    const c = cart('retail')
    expect(payIssue(c, copy)).toBe('')
    expect(effectivePayments(c, saleTotal(c))).toMatchObject([{ method: 'cash', amount: '120.00' }])
  })

  it('splits cash and card into two payments', () => {
    const c = pay(tillReducer(cart('retail'), { type: 'setQty', key: 1, qty: 10, lots: [] }), // two Bag 5kg = 1,100
      { method: 'cash', amount: '500' }, { method: 'card', amount: '600', accountId: '3' })
    expect(payIssue(c, copy)).toBe('')
    const b = saleBody(c, effectivePayments(c, saleTotal(c)))
    expect(b.payments).toMatchObject([{ method: 'cash', amount: '500' }, { method: 'card', amount: '600', accountId: 3 }])
  })

  it('asks for a customer before credit or a cheque, as the till does', () => {
    const c = cart('retail')
    expect(payIssue(pay(c, { method: 'credit', amount: '120' }), copy)).toBe('Pick a customer to sell on account')
    expect(payIssue(pay(c, { method: 'cheque', amount: '120', cheque: { bank: 'BOC', number: '1', date: '2026-11-01', note: '' } }), copy)).toBe('Pick the customer who gave the cheque')
    expect(payIssue({ ...pay(c, { method: 'credit', amount: '120' }), customerId: 4 }, copy)).toBe('')
  })

  it('needs the account for card and online payments', () => {
    const c = cart('retail')
    expect(payIssue(pay(c, { method: 'online', amount: '120' }), copy)).toBe('Choose the account for the online payment')
    expect(payIssue(pay(c, { method: 'card', amount: '120', accountId: '3' }), copy)).toBe('')
  })

  it('says what is still to pay instead of sending a short sale', () => {
    expect(payIssue(pay(cart('retail'), { method: 'cash', amount: '100' }), copy)).toBe('Rs. 20.00 still to pay')
  })
})
