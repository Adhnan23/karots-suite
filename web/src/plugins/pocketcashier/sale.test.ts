import { describe, expect, it } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState, tillReducer } from '@/features/till/cart/store'
import type { TillState } from '@/features/till/cart/types'
import { payIssue, payments, saleBody, type Tender } from './sale'

const rice = {
  id: 1, name: 'Rice', nameLocal: '', barcode: '', sellingPrice: '120.00', wholesalePrice: '100.00', taxRate: '0', stockQty: '50',
  unitAbbr: 'kg', unitAllowDecimal: true, isService: false, trackSerial: false, warrantyMonths: 0, lowStock: false, emoji: '',
  badges: [], packs: [{ id: 9, name: 'Bag 5kg', size: '5', sellingPrice: '550.00', wholesalePrice: '0.00' }], categoryId: 1, barcodes: [], packCodes: [],
} as unknown as CopyProduct

const copy = { products: [rice], places: [{ id: 3, name: 'Card machine', kind: 'card', cashierAccess: true }], customers: [] } as unknown as TillCopy

const cart = (type: 'retail' | 'wholesale', units?: number): TillState =>
  tillReducer(initialState(type), { type: 'add', product: rice, lots: [], units })

const cash = (given = ''): Tender => ({ method: 'cash', given, accountId: '', meta: {} })

describe('the phone sale', () => {
  it('keeps product ids and qty, sends the shelf price it showed, and never kicks the drawer', () => {
    const c = cart('retail', 5) // one Bag 5kg
    const b = saleBody(c, payments(c, cash()))
    expect(b.items).toHaveLength(1)
    expect(b.items[0]).toMatchObject({ productId: 1, quantity: '5', expectPrice: '120' })
    expect(b.deferDrawerKick).toBe(true)
    expect(b.payments[0]).toMatchObject({ method: 'cash' })
  })

  it('sends the wholesale price on a wholesale cart', () => {
    const c = cart('wholesale')
    expect(saleBody(c, payments(c, cash())).items[0].expectPrice).toBe('100')
  })

  it('asks for a customer before credit or a cheque, as the till does', () => {
    const c = cart('retail')
    expect(payIssue(c, { ...cash(), method: 'credit' }, copy)).toBe('Pick a customer to sell on account')
    expect(payIssue(c, { ...cash(), method: 'cheque' }, copy)).toBe('Pick the customer who gave the cheque')
    expect(payIssue({ ...c, customerId: 4 }, { ...cash(), method: 'credit' }, copy)).toBe('')
  })

  it('takes exact cash when nothing was keyed, and refuses less than the total', () => {
    const c = cart('retail')
    expect(payments(c, cash())[0].amount).toBe('120')
    expect(payIssue(c, cash('100'), copy)).toBe('The cash given is less than the total')
    expect(payIssue(c, cash('200'), copy)).toBe('')
  })

  it('puts a card payment in the card account', () => {
    const c = cart('retail')
    expect(payments(c, { method: 'card', given: '', accountId: '3', meta: {} })[0]).toMatchObject({ method: 'card', amount: '120', accountId: '3' })
  })
})
