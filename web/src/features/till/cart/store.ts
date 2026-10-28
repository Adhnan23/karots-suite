import type { HeldSale, Lot, TillProduct } from '@/api/types.gen'
import { addGroup, addServiceGroup, breakGroup, removeGroup, setGroupCount, type GroupInput } from './groups'
import { fromHeld } from './held'
import { addProduct, payoutLine, serviceLine, setQty, swapProduct, syncSerials } from './lines'
import { balanceDue, saleTotal } from './totals'
import type { DiscountType, LinePack, Payment, PayoutInput, SaleType, ServiceLineInput, TillState } from './types'

export const blankDraft = (method: string): Payment => ({ method, amount: '', reference: '', accountId: '', meta: {} })

export const initialState = (saleType: SaleType): TillState => ({
  lines: [], seq: 0, lastAddedKey: 0, saleType, customerId: null, billDiscount: '', billDiscountType: 'fixed', meta: {},
  tenders: [], draft: blankDraft('cash'), keep: '', roundToAccount: false, overLimitApproved: false,
  postSaleChange: 0, notice: null,
})

export type Action =
  | { type: 'add'; product: TillProduct; lot?: Lot; lots: Lot[]; units?: number; plugin?: Record<string, unknown>; discount?: { value: string; type: DiscountType } }
  | { type: 'saleMeta'; key: string; value: unknown }
  | { type: 'linePacks'; key: number; packs: LinePack[]; qty?: number }
  | { type: 'quickSet'; product: TillProduct; qty: number }
  | { type: 'addService'; line: ServiceLineInput }
  | { type: 'addPayout'; line: PayoutInput }
  | { type: 'addGroup'; group: GroupInput }
  | { type: 'addServiceGroup'; group: { label: string; lines: ServiceLineInput[] } }
  | { type: 'groupCount'; id: number; count: number }
  | { type: 'breakGroup'; id: number; key: number }
  | { type: 'removeGroup'; id: number }
  | { type: 'setQty'; key: number; qty: number; lots: Lot[] }
  | { type: 'remove'; key: number }
  | { type: 'swap'; key: number; product: TillProduct; qty?: number }
  | { type: 'lineDiscount'; key: number; discount: string; discountType: DiscountType }
  | { type: 'serial'; key: number; index: number; value: string }
  | { type: 'approveOversell'; key: number }
  | { type: 'approveConsumables' }
  | { type: 'saleType'; saleType: SaleType }
  | { type: 'customer'; id: number | null }
  | { type: 'billDiscount'; value?: string; discountType?: DiscountType }
  | { type: 'draft'; patch: Partial<Payment> }
  | { type: 'commitDraft' }
  | { type: 'addTender'; payment: Payment }
  | { type: 'removeTender'; index: number }
  | { type: 'removeCreditTenders' }
  | { type: 'keep'; value: string }
  | { type: 'roundToAccount'; on: boolean }
  | { type: 'approveOverLimit' }
  | { type: 'resume'; held: HeldSale }
  | { type: 'newSale'; saleType: SaleType; change?: number }
  | { type: 'clearPostSaleChange' }
  | { type: 'clearNotice' }
  | { type: 'restore'; state: TillState }

/** With a customer, don't pre-select cash (a blank line must route to "on account?",
 * never silently complete as paid). Only rewrites the untouched default draft. */
function syncDefaultTender(s: TillState): TillState {
  const d = s.draft
  if (s.tenders.length || d.amount !== '' || d.reference || (d.method !== '' && d.method !== 'cash')) return s
  return { ...s, draft: { ...d, method: s.customerId ? '' : 'cash' } }
}

const mapLine = (s: TillState, key: number, f: (l: TillState['lines'][number]) => TillState['lines'][number]) =>
  ({ ...s, lines: s.lines.map((l) => (l.key === key ? f(l) : l)) })

export function tillReducer(s: TillState, a: Action): TillState {
  switch (a.type) {
    case 'add': {
      const r = addProduct(s.lines, s.seq, a.product, s.saleType, a.lot, a.lots, a.units, a.plugin)
      // A plugin's discount lands on the line(s) just added: with plugin data, every line of
      // that product carrying it (lots may split it); otherwise the added line.
      const tag = JSON.stringify(a.plugin ?? null)
      const lines = !a.discount ? r.lines : r.lines.map((l) => (
        (a.plugin ? l.productId === a.product.id && JSON.stringify(l.plugin ?? null) === tag : l.key === r.key)
          ? { ...l, discount: a.discount!.value, discountType: a.discount!.type } : l))
      return { ...s, lines, seq: r.seq, lastAddedKey: r.key, notice: r.notice, postSaleChange: 0 }
    }
    case 'quickSet': {
      let st = s
      let hit = s.lines.find((l) => l.productId === a.product.id && !l.isService && !l.group)
      if (!hit) {
        const r = addProduct(s.lines, s.seq, a.product, s.saleType, undefined, [])
        st = { ...s, lines: r.lines, seq: r.seq, lastAddedKey: r.key }
        hit = r.lines.find((l) => l.key === r.key)
      }
      const r = setQty(st.lines, st.seq, hit!.key, a.qty, [])
      return { ...st, lines: r.lines, seq: r.seq, postSaleChange: 0 }
    }
    case 'addService': {
      const key = s.seq + 1
      return { ...s, lines: [...s.lines, serviceLine(key, a.line)], seq: key, lastAddedKey: key, postSaleChange: 0 }
    }
    case 'addPayout': {
      const key = s.seq + 1
      return { ...s, lines: [...s.lines, payoutLine(key, a.line)], seq: key, lastAddedKey: key, postSaleChange: 0 }
    }
    case 'addGroup': {
      const r = addGroup(s.lines, s.seq, s.saleType, a.group)
      return { ...s, lines: r.lines, seq: r.seq, lastAddedKey: r.seq, postSaleChange: 0 }
    }
    case 'addServiceGroup': {
      const r = addServiceGroup(s.lines, s.seq, a.group)
      return { ...s, lines: r.lines, seq: r.seq, lastAddedKey: r.seq, postSaleChange: 0 }
    }
    case 'groupCount':
      return { ...s, lines: setGroupCount(s.lines, a.id, a.count) }
    case 'breakGroup':
      return { ...s, lines: breakGroup(s.lines, a.id, a.key) }
    case 'removeGroup':
      return { ...s, lines: removeGroup(s.lines, a.id) }
    case 'setQty': {
      const r = setQty(s.lines, s.seq, a.key, a.qty, a.lots)
      return { ...s, lines: r.lines, seq: r.seq, notice: r.notice }
    }
    case 'remove':
      return { ...s, lines: s.lines.filter((l) => l.key !== a.key) }
    case 'swap':
      return { ...s, lines: swapProduct(s.lines, a.key, a.product, s.saleType, a.qty) }
    case 'saleMeta': {
      const meta = { ...(s.meta ?? {}) }
      if (a.value === undefined) delete meta[a.key]
      else meta[a.key] = a.value
      return { ...s, meta }
    }
    case 'linePacks': {
      const st = mapLine(s, a.key, (l) => ({ ...l, packs: a.packs }))
      if (a.qty === undefined) return st
      const r = setQty(st.lines, st.seq, a.key, a.qty, [])
      return { ...st, lines: r.lines, seq: r.seq, notice: r.notice }
    }
    case 'lineDiscount':
      return mapLine(s, a.key, (l) => ({ ...l, discount: a.discount, discountType: a.discountType }))
    case 'serial':
      return mapLine(s, a.key, (l) => syncSerials({ ...l, serials: l.serials.map((v, i) => (i === a.index ? a.value : v)) }))
    case 'approveOversell':
      return mapLine(s, a.key, (l) => ({ ...l, allowOversell: true }))
    case 'approveConsumables':
      return { ...s, lines: s.lines.map((l) => (l.isService || l.components.length ? { ...l, allowOversell: true } : l)) }
    case 'saleType':
      return { ...s, saleType: a.saleType }
    case 'customer': {
      let st: TillState = { ...s, customerId: a.id, overLimitApproved: false }
      if (a.id === null) st = { ...st, roundToAccount: false, draft: st.draft.method === 'credit' ? { ...st.draft, method: '' } : st.draft }
      return syncDefaultTender(st)
    }
    case 'billDiscount':
      return { ...s, billDiscount: a.value ?? s.billDiscount, billDiscountType: a.discountType ?? s.billDiscountType }
    case 'draft':
      return { ...s, draft: { ...s.draft, ...a.patch } }
    case 'commitDraft': {
      if (!(Number(s.draft.amount) > 0) || !s.draft.method) return s
      // Only cash gets change, so card / online / a plugin tender is capped at
      // what's still due (the server refuses more — sales.CheckTender).
      const due = balanceDue(saleTotal(s), s.tenders)
      const pay = s.draft.method !== 'cash' && s.draft.method !== 'credit' && Number(s.draft.amount) > due ? { ...s.draft, amount: due.toFixed(2) } : s.draft
      if (!(Number(pay.amount) > 0)) return s
      return { ...s, tenders: [...s.tenders, pay], draft: { ...blankDraft(s.draft.method), accountId: s.draft.accountId } }
    }
    case 'addTender':
      return { ...s, tenders: [...s.tenders, a.payment] }
    case 'removeTender':
      return syncDefaultTender({ ...s, tenders: s.tenders.filter((_, i) => i !== a.index) })
    case 'removeCreditTenders':
      return { ...s, tenders: s.tenders.filter((p) => p.method !== 'credit'), draft: s.draft.method === 'credit' ? blankDraft('') : s.draft }
    case 'keep':
      return { ...s, keep: a.value }
    case 'roundToAccount':
      return { ...s, roundToAccount: a.on }
    case 'approveOverLimit':
      return { ...s, overLimitApproved: true }
    case 'resume': {
      const r = fromHeld(a.held, s.seq)
      const saleType = a.held.saleType === 'wholesale' ? 'wholesale' : 'retail'
      return syncDefaultTender({
        ...initialState(saleType), seq: r.seq, lines: r.lines, customerId: a.held.customerId ?? null, meta: r.meta,
        billDiscount: Number(a.held.discount) ? String(Number(a.held.discount)) : '', billDiscountType: r.billDiscountType,
      })
    }
    case 'newSale':
      return { ...initialState(a.saleType), seq: s.seq, postSaleChange: a.change ?? 0 }
    case 'clearPostSaleChange':
      return { ...s, postSaleChange: 0 }
    case 'clearNotice':
      return { ...s, notice: null }
    case 'restore':
      return a.state
  }
}
