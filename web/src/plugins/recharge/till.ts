import type { TillProduct } from '@/api/types.gen'
import { queryClient } from '@/api/query-client'
import type { CartLine } from '@/features/till/cart/types'
import { num } from '@/lib/money'
import type { TillApi } from '../slots'
import { tillDataQuery } from './api'
import type { Carrier, Float, Kind, LineTag, Package, TillData } from './types'

/** A recharge cart line's tag, or null. */
export function tagOf(plugin: Record<string, unknown> | undefined): LineTag | null {
  const t = plugin?.recharge as LineTag | undefined
  return t && typeof t === 'object' && typeof t.kind === 'string' ? t : null
}

/** The carrier and own products ring only through the 📲 menu (a search hit opens it). */
export function claims(p: TillProduct): boolean {
  const d = queryClient.getQueryData(tillDataQuery.queryKey)
  if (!d) return false
  return d.carriers.some((c) => c.productId === p.id) || Object.values(d.products).includes(p.id)
}

/** The service charge a carrier's rate offers for a kind ('' = none). */
export function chargeFor(c: Carrier | undefined, kind: Kind): string {
  const v = num(c?.rates.find((r) => r.kind === kind)?.charge)
  return v > 0 ? String(v) : ''
}

export const floatName = (f: Float) => `${f.carrier} · ${f.label}`
/** Never stops a sale — but a float that would drop below zero is worth a word. */
export const goesBelow = (f: Float | undefined, amount: number) => !!f && num(f.balance) - amount < -0.005

type Account = { device?: Float; bank?: { id: number; name: string } }
const where = (a: Account): Pick<LineTag, 'device' | 'bank' | 'label'> =>
  a.device ? { device: a.device.id, label: floatName(a.device) } : { bank: a.bank!.id, label: a.bank!.name }
const tag = (t: LineTag) => ({ recharge: t })

/** A reload of amount from a device: one line on the carrier's pass-through product. */
export function addReload(till: TillApi, c: Carrier, f: Float, amount: number, number: string) {
  const n = number.trim()
  till.addServiceLine({ productId: c.productId, name: `${c.name} reload`, price: amount, fixedQty: true, noDiscount: true,
    description: n ? `${c.name} reload · ${n}` : `${c.name} reload`,
    plugin: tag({ kind: 'reload', device: f.id, label: floatName(f), ...(n && { number: n }) }) })
}

export function addPackage(till: TillApi, c: Carrier, f: Float, p: Package, number: string) {
  const n = number.trim()
  till.addServiceLine({ productId: c.productId, name: p.name, price: num(p.price), fixedQty: true,
    description: n ? `${c.name} ${p.name} · ${n}` : `${c.name} ${p.name}`,
    plugin: tag({ kind: 'package', device: f.id, package: p.id, label: floatName(f), ...(n && { number: n }) }) })
}

export function addCharge(till: TillApi, d: TillData, amount: number, what: string) {
  if (!(amount > 0)) return
  till.addServiceLine({ productId: d.products.charge, name: 'Service charge', price: amount, fixedQty: true,
    description: `Service charge · ${what}`, plugin: tag({ kind: 'charge' }) })
}

export function addBill(till: TillApi, d: TillData, a: Account, amount: number, billType: string, ref: string, charge: number) {
  const label = [billType.trim() || 'Bill payment', ref.trim()].filter(Boolean).join(' · ')
  till.addServiceLine({ productId: d.products.billpay, name: 'Bill payment', price: amount, fixedQty: true, noDiscount: true, description: label,
    plugin: tag({ kind: 'billpay', ...where(a), ...(billType.trim() && { billType: billType.trim() }), ...(ref.trim() && { ref: ref.trim() }) }) })
  addCharge(till, d, charge, billType.trim() || 'bill payment')
}

export function addDeposit(till: TillApi, d: TillData, f: Float, amount: number, number: string, charge: number) {
  const n = number.trim()
  till.addServiceLine({ productId: d.products.deposit, name: 'Wallet deposit', price: amount, fixedQty: true, noDiscount: true,
    description: `${f.carrier} wallet deposit${n ? ` · ${n}` : ''}`,
    plugin: tag({ kind: 'deposit', device: f.id, label: floatName(f), ...(n && { number: n }) }) })
  addCharge(till, d, charge, 'wallet deposit')
}

/** A cash out is a payout: the cart may go below zero and Pay gives the cash. */
export function addCashout(till: TillApi, d: TillData, a: Account, amount: number, charge: number) {
  const name = a.device ? `${a.device.carrier} cash out` : `Cash out · ${a.bank!.name}`
  till.addPayout({ productId: d.products.cashout, name: 'Cash out', amount, description: name, plugin: tag({ kind: 'cashout', ...where(a) }) })
  addCharge(till, d, charge, 'cash out')
}

/** The device / bank a cart's recharge lines draw on now, net of what's already in the cart. */
export function inCart(lines: CartLine[], deviceId: number): number {
  return lines.reduce((s, l) => {
    const t = tagOf(l.plugin)
    if (t?.device !== deviceId) return s
    return t.kind === 'cashout' ? s - num(l.priceOverride) : t.kind === 'charge' ? s : s + num(l.unitPrice) * l.qty
  }, 0)
}
