import { describe, expect, it } from 'vitest'
import type { SaleLine } from '@/api/types.gen'
import type { SaleView } from '@/api/types.gen'
import { bumpQty, left, refundOf, refundSplit } from './refund'

const line = (id: number, quantity: string, group = 0, sets = '', returnedQty = '0'): SaleLine => ({
  id, name: `L${id}`, unitAbbr: 'pc', quantity, returnedQty, unitPrice: '10.00', subtotal: '100.00', refundUnit: '10.00',
  paid: '100.00', refundLeft: '100.00', isService: false, group, groupLabel: group ? 'Combo' : '', groupSets: sets, packLabel: '' })

// RET-13: a combo comes back whole, in whole sets — its lines move together.
describe('bumpQty', () => {
  const lines = [line(1, '2', 1, '2'), line(2, '4', 1, '2'), line(3, '5')]

  it('moves a plain line one unit, within what is left', () => {
    expect(bumpQty(lines, {}, lines[2], 1)).toEqual({ 3: 1 })
    expect(bumpQty(lines, { 3: 5 }, lines[2], 1)).toEqual({ 3: 5 })
    expect(bumpQty(lines, { 3: 0 }, lines[2], -1)).toEqual({ 3: 0 })
  })

  it('moves every line of a combo one whole set', () => {
    expect(bumpQty(lines, {}, lines[0], 1)).toEqual({ 1: 1, 2: 2 })
    expect(bumpQty(lines, { 1: 1, 2: 2 }, lines[1], 1)).toEqual({ 1: 2, 2: 4 })
    expect(bumpQty(lines, { 1: 2, 2: 4 }, lines[0], 1)).toEqual({ 1: 2, 2: 4 })
    expect(bumpQty(lines, { 1: 2, 2: 4 }, lines[1], -1)).toEqual({ 1: 1, 2: 2 })
  })

  it('stops at the sets still out', () => {
    const half = [line(1, '2', 1, '2', '1'), line(2, '4', 1, '2', '2')]
    expect(bumpQty(half, { 1: 1, 2: 2 }, half[0], 1)).toEqual({ 1: 1, 2: 2 })
  })
})

// TILL-2: the return screen's refund for n units = the server's (sales.lineRefund:
// paid × n ÷ qty, rounded half away from zero to the cent, in exact decimals).
describe('refundOf agrees with the server to the cent', () => {
  it('for every paid value up to Rs 500 and every part of a line up to 9', () => {
    const bad: string[] = []
    for (let cents = 1; cents <= 50000; cents += 1) {
      for (let q = 2; q <= 9; q++) {
        for (let n = 1; n < q; n++) {
          const want = Math.floor((2 * cents * n + q) / (2 * q)) // round half up of cents·n/q, in integers
          const l = line(1, String(q))
          l.paid = (cents / 100).toFixed(2)
          const got = Math.round(refundOf(l, n) * 100)
          if (got !== want && bad.length < 5) bad.push(`paid ${l.paid}, ${n} of ${q}: ${got / 100} ≠ ${want / 100}`)
        }
      }
    }
    expect(bad).toEqual([])
  })
})

// RET-17 + RET-18: how a refund splits — debt on the sale first, then (if the
// cashier gives the uncleared cheque back) the cheque, the rest in cash.
describe('refundSplit', () => {
  const sale = (owed: string, cheques: { amount: string; chequeStatus: string }[]) =>
    ({ owedOnSale: owed, payments: cheques.map((c) => ({ method: 'cheque', ...c })) }) as unknown as SaleView

  it('pays cash when there is no cheque waiting', () => {
    expect(refundSplit(sale('0.00', []), 300, 'cash')).toEqual({ offAccount: 0, chequeBack: 0, cash: 300, pendingCheque: 0 })
    expect(refundSplit(sale('0.00', [{ amount: '1000.00', chequeStatus: 'cleared' }]), 300, 'back').cash).toBe(300)
  })
  it('gives the cheque back instead of cash', () => {
    expect(refundSplit(sale('0.00', [{ amount: '1000.00', chequeStatus: 'pending' }]), 400, 'back')).toEqual({ offAccount: 0, chequeBack: 400, cash: 0, pendingCheque: 1000 })
    expect(refundSplit(sale('0.00', [{ amount: '300.00', chequeStatus: 'pending' }]), 400, 'back')).toEqual({ offAccount: 0, chequeBack: 300, cash: 100, pendingCheque: 300 })
  })
  it('keeps the cheque and pays cash when asked', () => {
    expect(refundSplit(sale('0.00', [{ amount: '1000.00', chequeStatus: 'pending' }]), 400, 'cash').cash).toBe(400)
  })
  it('clears debt on the sale first', () => {
    expect(refundSplit(sale('150.00', [{ amount: '1000.00', chequeStatus: 'pending' }]), 400, 'back')).toEqual({ offAccount: 150, chequeBack: 250, cash: 0, pendingCheque: 1000 })
  })
})

// A free line (100% off) can still come back — its stock goes back on the shelf;
// only a payout (money handed over) never does.
describe('left', () => {
  it('counts a free line, never a payout', () => {
    expect(left({ ...line(1, '3'), subtotal: '0.00' })).toBe(3)
    expect(left({ ...line(1, '1'), subtotal: '-500.00' })).toBe(0)
  })
})
