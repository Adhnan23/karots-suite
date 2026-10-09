import type { SaleRequest, TillCustomer } from '@/api/types.gen'
import { dec, num, r2 } from '@/lib/money'
import { accountTotal, actualChange, changeDue, keptAmount, paidTotal, roundable, saleTotal, tenderIssue } from './totals'
import type { Payment, TillState } from './types'

export type Prompt =
  | { kind: 'oversell'; key: number; name: string; stock: number; qty: number }
  | { kind: 'shortfall'; amount: number; hasCustomer: boolean }
  | { kind: 'overcovered'; amount: number; hasCustomer: boolean }
  | { kind: 'overlimit'; amount: number; over: number }

export type GateCtx = {
  customer: TillCustomer | null
  canManageCredit: boolean
  confirmed: boolean // the tender prompt already answered (cannot loop)
}

export type Gate = { prompt: Prompt } | { error: string } | { ok: true; payments: Payment[] }

/** Committed tenders + the draft line. A walk-in's untouched cash draft means
 * "exact cash" so the cashier can just hit Complete. */
export function effectivePayments(s: TillState, due: number): Payment[] {
  const ps = [...s.tenders]
  if (num(s.draft.amount) > 0 || (s.draft.amount !== '' && !s.draft.method)) ps.push(s.draft)
  if (ps.length === 0 && s.draft.method === 'cash' && due > 0) ps.push({ ...s.draft, amount: dec(due) })
  return ps
}

/** Everything that must be true before posting, in karots-pos's order. */
export function checkoutGate(s: TillState, ctx: GateCtx): Gate {
  if (!s.lines.length) return { error: 'Cart is empty' }
  const t = saleTotal(s)
  const payments = effectivePayments(s, t)
  const short = s.lines.find((l) => !l.allowOversell && l.qty > l.stock + 1e-9)
  if (short) return { prompt: { kind: 'oversell', key: short.key, name: short.name, stock: short.stock, qty: short.qty } }
  if (!ctx.confirmed) {
    const issue = tenderIssue(t, paidTotal(payments), accountTotal(payments), s.customerId !== null)
    if (issue) return { prompt: issue }
  }
  if (!s.overLimitApproved && ctx.customer && ctx.canManageCredit) {
    const onAcct = accountTotal(payments)
    const avail = num(ctx.customer.creditLimit) - num(ctx.customer.outstanding)
    if (onAcct > 0.005 && onAcct > avail + 0.005) return { prompt: { kind: 'overlimit', amount: r2(onAcct), over: r2(onAcct - avail) } }
  }
  for (const l of s.lines) {
    if (!l.trackSerial) continue
    const n = Math.max(0, Math.round(l.qty))
    if (l.serials.filter((x) => x.trim() !== '').length < n) return { error: `Enter ${n} serial number(s) for ${l.name}` }
  }
  if (payments.some((p) => num(p.amount) > 0 && !p.method)) return { error: 'Choose a payment method for each amount' }
  for (const p of payments) {
    if ((p.method === 'card' || p.method === 'online') && num(p.amount) > 0 && !p.accountId) {
      return { error: `Choose the account for the ${p.method} payment` }
    }
  }
  return { ok: true, payments }
}

export function buildSaleRequest(s: TillState, payments: Payment[], opts: { deferDrawerKick: boolean }): SaleRequest {
  const t = saleTotal(s)
  const change = changeDue(t, payments)
  const kept = roundable(t, payments) ? keptAmount(s.keep, change) : 0
  return {
    saleType: s.saleType,
    customerId: s.customerId ?? undefined,
    discount: String(num(s.billDiscount)),
    discountType: s.billDiscountType,
    allowOverLimit: s.overLimitApproved,
    deferDrawerKick: opts.deferDrawerKick,
    changeGiven: kept > 0.005 ? dec(actualChange(change, kept)) : '',
    onAccountRounding: kept > 0.005 && s.customerId !== null && s.roundToAccount,
    items: s.lines.map((l) => ({
      productId: l.productId, quantity: String(l.qty), discount: String(num(l.discount)), discountType: l.discountType,
      priceOverride: l.priceOverride, batchId: l.batchId, allowOversell: l.allowOversell,
      serials: l.trackSerial ? l.serials.map((x) => x.trim()) : [], description: l.description, components: l.components,
    })),
    payments: payments.filter((p) => num(p.amount) > 0).map((p) => ({
      method: p.method, amount: String(num(p.amount)), reference: p.reference, accountId: num(p.accountId),
      meta: Object.keys(p.meta).length ? p.meta : undefined,
    })),
  }
}
