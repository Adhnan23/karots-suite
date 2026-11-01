import type { SaleRequest, TillCopy } from '@/api/types.gen'
import { buildSaleRequest, chequeMissing, effectivePayments } from '@/features/till/cart/checkout'
import { accountTotal, paidTotal, saleTotal, tenderIssue } from '@/features/till/cart/totals'
import type { Payment, TillState } from '@/features/till/cart/types'
import { dec, money, num } from '@/lib/money'

/** What stops this sale, as the till would say it ('' = ready), over the till's
 * own tenders (committed + the draft, effectivePayments). Stock, serials and
 * credit limits never stop a phone sale: the server takes it and flags it. */
export function payIssue(cart: TillState, copy: TillCopy): string {
  if (!cart.lines.length) return 'Cart is empty'
  const t = saleTotal(cart)
  const ps = effectivePayments(cart, t).filter((p) => num(p.amount) > 0)
  if (ps.some((p) => !p.method)) return 'Choose a payment method for each amount'
  if (ps.some((p) => p.method === 'credit') && cart.customerId === null) return 'Pick a customer to sell on account'
  const cheques = ps.filter((p) => p.method === 'cheque')
  if (cheques.length) {
    if (cart.customerId === null) return 'Pick the customer who gave the cheque'
    const missing = cheques.map((p) => chequeMissing(p.cheque)).find(Boolean)
    if (missing) return missing
  }
  for (const p of ps) {
    const kind = p.method === 'card' ? 'card' : p.method === 'online' ? 'bank' : ''
    if (kind && !copy.places.some((x) => x.kind === kind && String(x.id) === p.accountId)) return `Choose the account for the ${p.method} payment`
  }
  if (t < 0 && ps.some((p) => p.method !== 'cash')) return 'Money given to the customer goes out in cash — take off the other payments'
  const nonCash = ps.filter((p) => p.method !== 'cash' && p.method !== 'credit').reduce((a, p) => a + num(p.amount), 0)
  if (nonCash > t + 0.005) return cheques.length ? "A cheque can't be more than what's due — only cash gets change" : "Only cash gets change — take no more than what's due"
  const issue = tenderIssue(t, paidTotal(ps), accountTotal(ps), cart.customerId !== null)
  if (issue?.kind === 'shortfall') return `${money(copy.symbol, issue.amount)} still to pay${issue.hasCustomer ? ' — add it on account if they owe it' : ''}`
  if (issue?.kind === 'overcovered') return 'More than the total is paid — take off the on-account line'
  return ''
}

/** The sale action's body: the till's own request, the drawer never kicked, and
 * each line's unit price as the phone showed it (the server flags a difference).
 * total is the phone's own (the drawer's expected cash counts change from it). */
export function saleBody(cart: TillState, ps: Payment[]): SaleRequest & { total: string; items: { expectPrice: string }[] } {
  const req = buildSaleRequest(cart, ps, { deferDrawerKick: true })
  return { ...req, total: dec(saleTotal(cart)), items: req.items.map((it, i) => ({ ...it, expectPrice: cart.lines[i].payout ? '' : String(cart.lines[i].unitPrice) })) }
}
