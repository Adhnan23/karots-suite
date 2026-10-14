import { num, r2 } from '@/lib/money'
import { packed, packGross } from './packs'
import type { CartLine, DiscountType, Payment, TillState } from './types'

export const lineGross = (l: CartLine) =>
  packed(l) ? packGross(num(l.qty), num(l.unitPrice), l.packs!).gross : num(l.qty) * num(l.unitPrice)

/** Fixed is per unit (× qty); percent is off the line gross; clamped to [0, gross]. */
export function lineDiscount(l: CartLine) {
  const g = lineGross(l)
  const v = num(l.discount)
  const d = l.discountType === 'percent' ? (g * v) / 100 : v * num(l.qty)
  return Math.min(Math.max(d, 0), g)
}

export const lineNet = (l: CartLine) => lineGross(l) - lineDiscount(l)
export const subtotal = (ls: CartLine[]) => ls.reduce((s, l) => s + lineGross(l), 0)
export const itemDiscountTotal = (ls: CartLine[]) => ls.reduce((s, l) => s + lineDiscount(l), 0)

/** Bill discount resolved against the net after item discounts. */
export function billDiscount(ls: CartLine[], value: string, type: DiscountType) {
  const base = subtotal(ls) - itemDiscountTotal(ls)
  const v = num(value)
  const d = type === 'percent' ? (base * v) / 100 : v
  return Math.min(Math.max(d, 0), base)
}

/** Tax on each discounted line net (the server's rule). */
export const taxTotal = (ls: CartLine[]) => ls.reduce((s, l) => s + (lineNet(l) * l.taxRate) / 100, 0)

export const total = (ls: CartLine[], value: string, type: DiscountType) =>
  r2(Math.max(0, subtotal(ls) - itemDiscountTotal(ls) - billDiscount(ls, value, type) + taxTotal(ls)))

export const saleTotal = (s: TillState) => total(s.lines, s.billDiscount, s.billDiscountType)

/** Money actually handed over. An on-account line is a debt, not money. */
export const paidTotal = (ps: Payment[]) => ps.filter((p) => p.method !== 'credit').reduce((s, p) => s + num(p.amount), 0)
export const accountTotal = (ps: Payment[]) => ps.filter((p) => p.method === 'credit').reduce((s, p) => s + num(p.amount), 0)
export const changeDue = (t: number, ps: Payment[]) => r2(Math.max(0, paidTotal(ps) + accountTotal(ps) - t))
export const balanceDue = (t: number, ps: Payment[]) => r2(Math.max(0, t - paidTotal(ps) - accountTotal(ps)))

/** Rounding only on a pure cash-change sale (the server zeroes change with an account line). */
export const roundable = (t: number, ps: Payment[]) => accountTotal(ps) === 0 && changeDue(t, ps) > 0.005

/** What the shop keeps: the cashier's entry clamped to [0, change]; blank = nothing. */
export function keptAmount(keep: string, change: number) {
  if (keep.trim() === '') return 0
  return Math.min(change, Math.max(0, num(keep)))
}
export const actualChange = (change: number, kept: number) => r2(Math.max(0, change - kept))

/** The "Keep Rs x (no coins)" suggestion: the part of the change under Rs 10. */
export const coinKeep = (change: number) => r2(change % 10)

/** Greedy high→low note breakdown in whole cents (advisory only). */
export function notesFor(amount: number, denoms: number[]) {
  let rem = Math.round(amount * 100)
  const out: { value: number; qty: number }[] = []
  for (const v of [...denoms].sort((a, b) => b - a)) {
    const c = Math.round(v * 100)
    if (c <= 0) continue
    const qty = Math.floor(rem / c)
    if (qty > 0) {
      out.push({ value: v, qty })
      rem -= qty * c
    }
  }
  return out
}

export type TenderIssue = { kind: 'shortfall' | 'overcovered'; amount: number; hasCustomer: boolean } | null

/** The whole tender rule in one place (karots-pos pos-payment.js tenderIssue). */
export function tenderIssue(t: number, paid: number, onAccount: number, hasCustomer: boolean): TenderIssue {
  const covered = paid + onAccount
  if (covered < t - 0.005) return { kind: 'shortfall', amount: r2(t - covered), hasCustomer }
  if (onAccount > 0 && covered > t + 0.005) return { kind: 'overcovered', amount: r2(onAccount), hasCustomer }
  return null
}

/** Quick-cash chips: exact, then the next 500 / 1000, then 5000 — only ≥ the due. */
export function quickCash(due: number): number[] {
  if (due <= 0) return []
  const c = [r2(due), Math.ceil(due / 500) * 500, Math.ceil(due / 1000) * 1000, 5000]
  return [...new Set(c)].filter((v) => v >= due).slice(0, 4)
}
