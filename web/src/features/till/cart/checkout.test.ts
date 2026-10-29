import { describe, expect, it } from 'vitest'
import type { TillCustomer } from '@/api/types.gen'
import { buildSaleRequest, checkoutGate, effectivePayments, type GateCtx } from './checkout'
import { initialState, tillReducer } from './store'
import type { TillState } from './types'

const withLine = (s: TillState, over: Partial<TillState['lines'][number]> = {}) => {
  const t = tillReducer(s, { type: 'addService', line: { productId: 1, name: 'Item', price: 100 } })
  return { ...t, lines: t.lines.map((l) => ({ ...l, isService: false, stock: 10, priceOverride: '', ...over })) }
}
const cust: TillCustomer = { id: 5, name: 'Nimal', phone: '077', creditLimit: '1000.00', outstanding: '950.00' }
const ctx = (p: Partial<GateCtx> = {}): GateCtx => ({ customer: null, canManageCredit: false, confirmed: false, ...p })

describe('checkoutGate', () => {
  it('walk-in with a blank cash draft = exact cash', () => {
    const s = withLine(initialState('retail'))
    const g = checkoutGate(s, ctx())
    expect('ok' in g && g.payments).toEqual([expect.objectContaining({ method: 'cash', amount: '100.00' })])
  })
  it('customer + blank tender never completes as cash: it asks to put it on account', () => {
    const s = withLine(tillReducer(initialState('retail'), { type: 'customer', id: 5 }))
    expect(checkoutGate(s, ctx({ customer: cust }))).toEqual({ prompt: { kind: 'shortfall', amount: 100, hasCustomer: true } })
  })
  it('found-at-till is asked first, one line at a time', () => {
    const s = withLine(initialState('retail'), { qty: 11 })
    expect(checkoutGate(s, ctx())).toMatchObject({ prompt: { kind: 'oversell', qty: 11, stock: 10 } })
  })
  it('over-limit is offered only to users who may approve it', () => {
    let s = withLine(tillReducer(initialState('retail'), { type: 'customer', id: 5 }))
    s = tillReducer(s, { type: 'addTender', payment: { method: 'credit', amount: '100', reference: '', accountId: '', meta: {} } })
    expect(checkoutGate(s, ctx({ customer: cust, canManageCredit: true }))).toEqual({ prompt: { kind: 'overlimit', amount: 100, over: 50 } })
    expect('ok' in checkoutGate(s, ctx({ customer: cust }))).toBe(true) // server is the backstop
  })
  it('a confirmed shortfall passes the tender check', () => {
    const s = withLine(tillReducer(initialState('retail'), { type: 'draft', patch: { amount: '40' } }))
    expect('prompt' in checkoutGate(s, ctx())).toBe(true)
    expect('ok' in checkoutGate(s, ctx({ confirmed: true }))).toBe(true)
  })
  it('card always needs its account; serials must be complete', () => {
    let s = withLine(initialState('retail'))
    s = tillReducer(s, { type: 'draft', patch: { method: 'card', amount: '100' } })
    expect(checkoutGate(s, ctx())).toEqual({ error: 'Choose the account for the card payment' })
    const t = withLine(initialState('retail'), { trackSerial: true, serials: [''] })
    expect(checkoutGate(t, ctx())).toEqual({ error: 'Enter 1 serial number(s) for Item' })
  })
  it('an amount with no method is refused', () => {
    const s = withLine(tillReducer(initialState('retail'), { type: 'draft', patch: { method: '', amount: '100' } }))
    expect(checkoutGate(s, ctx())).toEqual({ error: 'Choose a payment method for each amount' })
  })
})

describe('buildSaleRequest', () => {
  it('sends the kept change only when the shop keeps something', () => {
    let s = withLine(initialState('retail'))
    s = tillReducer(s, { type: 'addTender', payment: { method: 'cash', amount: '150', reference: '', accountId: '', meta: {} } })
    const pays = effectivePayments(s, 100)
    expect(buildSaleRequest(s, pays, { deferDrawerKick: true }).changeGiven).toBe('')
    s = tillReducer(s, { type: 'keep', value: '5' })
    const req = buildSaleRequest(s, pays, { deferDrawerKick: true })
    expect(req).toMatchObject({ changeGiven: '45.00', deferDrawerKick: true, onAccountRounding: false })
    expect(req.items[0]).toMatchObject({ productId: 1, quantity: '1', batchId: 0 })
  })
})

describe('only cash gets change', () => {
  it('caps a card amount at what is due, leaves cash alone', () => {
    let s = withLine(initialState('retail'))
    s = tillReducer(s, { type: 'draft', patch: { method: 'card', amount: '5000' } })
    s = tillReducer(s, { type: 'commitDraft' })
    expect(s.tenders.map((t) => [t.method, t.amount])).toEqual([['card', '100.00']])
    s = tillReducer(withLine(initialState('retail')), { type: 'draft', patch: { method: 'cash', amount: '5000' } })
    s = tillReducer(s, { type: 'commitDraft' })
    expect(s.tenders.map((t) => [t.method, t.amount])).toEqual([['cash', '5000']])
  })
})

describe('buildSaleRequest plugin data', () => {
  it('sends a line\'s plugin data with the sale (for the plugins\' in-sale hooks); plain lines send none', () => {
    const two = withLine(withLine(initialState('retail')))
    const s = { ...two, lines: two.lines.map((l, i) => (i === 1 ? { ...l, plugin: { repairs: { job: 7, ticket: 'R-0007' } } } : l)) }
    const req = buildSaleRequest(s, [{ method: 'cash', amount: '200', reference: '', accountId: '', meta: {} }], { deferDrawerKick: true })
    expect(req.items.map((i) => i.plugin)).toEqual([undefined, { repairs: { job: 7, ticket: 'R-0007' } }])
  })
})

describe('line groups on the sale request', () => {
  it('a service group posts itemised (the receipt prices each line); a combo does not', () => {
    const s = tillReducer(initialState('retail'), { type: 'addServiceGroup', group: { label: 'Photocopy', lines: [
      { productId: 70, name: 'Photocopy A4', price: 40, qty: 4, unitPrice: 10 }, { productId: 70, name: 'Urgent', price: 100 }] } })
    const req = buildSaleRequest(s, [], { deferDrawerKick: false })
    expect(req.items.map((i) => i.group)).toEqual([
      { no: 1, label: 'Photocopy', sets: '1', itemised: true }, { no: 1, label: 'Photocopy', sets: '1', itemised: true }])
  })
})

describe('cheque tender', () => {
  const chq = (amount: string, over: Partial<{ bank: string; number: string; date: string; note: string }> = {}) => ({
    method: 'cheque', amount, reference: '', accountId: '', meta: {},
    cheque: { bank: 'Bank of Ceylon', number: '123456', date: '2026-10-29', note: '', ...over },
  })
  const named = () => withLine(tillReducer(initialState('retail'), { type: 'customer', id: 5 }))

  it('needs a named customer', () => {
    const s = tillReducer(withLine(initialState('retail')), { type: 'addTender', payment: chq('100') })
    expect(checkoutGate(s, ctx({ confirmed: true }))).toEqual({ error: 'Pick the customer who gave the cheque' })
  })
  it('needs its number and date', () => {
    const s = tillReducer(named(), { type: 'addTender', payment: chq('100', { number: ' ' }) })
    expect(checkoutGate(s, ctx({ customer: cust, confirmed: true }))).toEqual({ error: 'Enter the cheque number and date' })
  })
  it('needs its bank', () => {
    const s = tillReducer(named(), { type: 'addTender', payment: chq('100', { bank: ' ' }) })
    expect(checkoutGate(s, ctx({ customer: cust, confirmed: true }))).toEqual({ error: "Enter the cheque's bank" })
  })
  it("can't pay more than what's due (no change on a cheque)", () => {
    const s = tillReducer(named(), { type: 'draft', patch: chq('150') })
    expect(checkoutGate(s, ctx({ customer: cust, confirmed: true }))).toEqual({ error: "A cheque can't be more than what's due — only cash gets change" })
  })
  it('is capped at the balance when added, like card', () => {
    let s = tillReducer(named(), { type: 'addTender', payment: { method: 'cash', amount: '40', reference: '', accountId: '', meta: {} } })
    s = tillReducer(s, { type: 'draft', patch: chq('100') })
    s = tillReducer(s, { type: 'commitDraft' })
    expect(s.tenders[1]).toMatchObject({ method: 'cheque', amount: '60.00' })
  })
  it('sends the cheque with its line, and nothing extra for cash', () => {
    let s = tillReducer(named(), { type: 'addTender', payment: chq('60') })
    s = tillReducer(s, { type: 'addTender', payment: { method: 'cash', amount: '40', reference: '', accountId: '', meta: {} } })
    const g = checkoutGate(s, ctx({ customer: cust }))
    if (!('ok' in g)) throw new Error(JSON.stringify(g))
    const req = buildSaleRequest(s, g.payments, { deferDrawerKick: false })
    expect(req.payments[0]).toMatchObject({ method: 'cheque', amount: '60', cheque: { bank: 'Bank of Ceylon', number: '123456', date: '2026-10-29' } })
    expect(req.payments[1].cheque).toBeUndefined()
    expect(req.changeGiven).toBe('')
  })
})
