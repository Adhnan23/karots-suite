import type { SaleRequest, TillCopy } from '@/api/types.gen'
import { buildSaleRequest, chequeMissing } from '@/features/till/cart/checkout'
import { saleTotal } from '@/features/till/cart/totals'
import type { ChequeDraft, Payment, TillState } from '@/features/till/cart/types'
import { dec, num } from '@/lib/money'

/** The one tender a phone sale is paid with. given = the cash handed over ('' = exact). */
export type Tender = { method: string; given: string; accountId: string; cheque?: ChequeDraft; meta: Record<string, string> }

/** The tender as the till's payments: cash is what was handed over (change comes
 * back), credit is the whole sale on account, anything else is the total. */
export function payments(cart: TillState, t: Tender): Payment[] {
  const total = saleTotal(cart)
  const amount = t.method === 'cash' && t.given.trim() !== '' ? String(num(t.given)) : dec(total)
  return [{ method: t.method, amount: String(num(amount)), reference: '', accountId: t.accountId, meta: t.meta, cheque: t.method === 'cheque' ? t.cheque : undefined }]
}

/** What stops this tender, as the till would say it ('' = ready). Stock and
 * credit limits never stop a phone sale: the server takes it and flags it. */
export function payIssue(cart: TillState, t: Tender, copy: TillCopy): string {
  if (!cart.lines.length) return 'Cart is empty'
  if (t.method === 'credit' && cart.customerId === null) return 'Pick a customer to sell on account'
  if (t.method === 'cheque') return cart.customerId === null ? 'Pick the customer who gave the cheque' : chequeMissing(t.cheque)
  if (t.method === 'card' && !copy.places.some((p) => p.kind === 'card' && String(p.id) === t.accountId)) return 'Choose the account for the card payment'
  if (t.method === 'cash' && t.given.trim() !== '' && num(t.given) < saleTotal(cart) - 0.005) return 'The cash given is less than the total'
  return ''
}

/** The sale action's body: the till's own request, the drawer never kicked, and
 * each line's unit price as the phone showed it (the server flags a difference).
 * total is the phone's own (the drawer's expected cash counts change from it). */
export function saleBody(cart: TillState, ps: Payment[]): SaleRequest & { total: string; items: { expectPrice: string }[] } {
  const req = buildSaleRequest(cart, ps, { deferDrawerKick: true })
  return { ...req, total: dec(saleTotal(cart)), items: req.items.map((it, i) => ({ ...it, expectPrice: cart.lines[i].payout ? '' : String(cart.lines[i].unitPrice) })) }
}
