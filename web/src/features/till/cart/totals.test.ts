import { describe, expect, it } from 'vitest'
import type { CartLine, Payment } from './types'
import {
  accountTotal, billDiscount, changeDue, coinKeep, keptAmount, lineDiscount, lineNet, notesFor,
  paidTotal, quickCash, roundable, taxTotal, tenderIssue, total,
} from './totals'

const line = (p: Partial<CartLine>): CartLine => ({
  key: 1, productId: 1, name: 'x', unitPrice: 100, taxRate: 0, qty: 1, stock: 10, isService: false,
  allowDecimal: false, batchId: 0, batchRemaining: 0, batchLabel: '', allowOversell: false,
  discount: '', discountType: 'fixed', trackSerial: false, warrantyMonths: 0, serials: [],
  priceOverride: '', description: '', components: [], ...p,
})
const pay = (method: string, amount: string): Payment => ({ method, amount, reference: '', accountId: '', meta: {} })

describe('line maths (mirrors the server)', () => {
  it('fixed discount is per unit', () => {
    const l = line({ qty: 2, discount: '10' })
    expect(lineDiscount(l)).toBe(20)
    expect(lineNet(l)).toBe(180)
  })
  it('percent discount is off the line gross', () => {
    expect(lineDiscount(line({ qty: 2, discount: '10', discountType: 'percent' }))).toBe(20)
  })
  it('a discount never exceeds the line', () => {
    expect(lineDiscount(line({ discount: '500' }))).toBe(100)
    expect(lineDiscount(line({ discount: '-5' }))).toBe(0)
  })
})

describe('bill totals', () => {
  const ls = [line({ qty: 2, discount: '10' }), line({ key: 2, unitPrice: 50, taxRate: 10 })]
  it('bill percent applies to the net after item discounts', () => {
    expect(billDiscount(ls, '10', 'percent')).toBe(23) // (200-20+50) * 10%
  })
  it('tax is on each line net, before the bill discount (server rule)', () => {
    expect(taxTotal(ls)).toBe(5)
  })
  it('total = gross − item disc − bill disc + tax, never negative', () => {
    expect(total(ls, '10', 'percent')).toBe(212) // 250 - 20 - 23 + 5
    expect(total(ls, '9999', 'fixed')).toBe(5)
  })
})

describe('tenders', () => {
  it('an on-account line is a debt, not money: excluded from paid, no change on it', () => {
    const ps = [pay('cash', '50'), pay('credit', '50')]
    expect(paidTotal(ps)).toBe(50)
    expect(accountTotal(ps)).toBe(50)
    expect(changeDue(100, ps)).toBe(0)
    expect(roundable(100, [pay('cash', '150')])).toBe(true)
    expect(roundable(100, [pay('cash', '150'), pay('credit', '1')])).toBe(false)
  })
  it('kept amount is clamped to [0, change]; blank keeps nothing', () => {
    expect(keptAmount('', 50)).toBe(0)
    expect(keptAmount('80', 50)).toBe(50)
    expect(keptAmount('-3', 50)).toBe(0)
    expect(coinKeep(47.5)).toBe(7.5)
  })
  it('tenderIssue: shortfall, over-covered by an account line, or fine', () => {
    expect(tenderIssue(100, 60, 0, true)).toEqual({ kind: 'shortfall', amount: 40, hasCustomer: true })
    expect(tenderIssue(100, 80, 50, true)).toEqual({ kind: 'overcovered', amount: 50, hasCustomer: true })
    expect(tenderIssue(100, 150, 0, false)).toBeNull()
  })
  it('notesFor breaks change down high→low in whole cents', () => {
    expect(notesFor(1270, [5000, 1000, 500, 100, 50, 20, 10, 5, 2, 1])).toEqual([
      { value: 1000, qty: 1 }, { value: 100, qty: 2 }, { value: 50, qty: 1 }, { value: 20, qty: 1 },
    ])
  })
  it('quickCash offers exact then round notes at or above the due', () => {
    expect(quickCash(1270)).toEqual([1270, 1500, 2000, 5000])
    expect(quickCash(6200)).toEqual([6200, 6500, 7000])
    expect(quickCash(0)).toEqual([])
  })
})
