import type { TillProduct } from '@/api/types.gen'
import { newLine, syncSerials } from './lines'
import type { CartLine, SaleType } from './types'

/** One member of a group: its product, its qty in ONE set and a fixed per-unit discount. */
export type GroupItem = { product: TillProduct; per: number; discount: string }
/** Units to take off existing plain lines (a hint turning scanned items into the group). */
export type Take = { key: number; qty: number }
export type GroupInput = { label: string; count: number; items: GroupItem[]; take?: Take[]; plugin?: Record<string, unknown> }

/** Take units off plain lines (a line at 0 goes), then add the group's lines, in one step. */
export function addGroup(lines: CartLine[], seq: number, saleType: SaleType, g: GroupInput): { lines: CartLine[]; seq: number } {
  let next = lines
  for (const t of g.take ?? []) {
    next = next
      .map((l) => (l.key === t.key && !l.group ? syncSerials({ ...l, qty: l.qty - t.qty }) : l))
      .filter((l) => l.qty > 0)
  }
  const count = Math.max(1, Math.round(g.count))
  const id = ++seq
  const added = g.items.map((it) => {
    const l = newLine(++seq, it.product, saleType)
    return syncSerials({
      ...l, qty: it.per * count, discount: it.discount, discountType: 'fixed' as const,
      group: { id, label: g.label, per: it.per, count }, plugin: g.plugin,
    })
  })
  return { lines: [...next, ...added], seq }
}

/** Set how many sets: every line follows; 0 removes the group. */
export function setGroupCount(lines: CartLine[], id: number, count: number): CartLine[] {
  const n = Math.max(0, Math.round(count))
  if (n === 0) return removeGroup(lines, id)
  return lines.map((l) => (l.group?.id === id ? syncSerials({ ...l, qty: l.group.per * n, group: { ...l.group, count: n } }) : l))
}

export const removeGroup = (lines: CartLine[], id: number) => lines.filter((l) => l.group?.id !== id)

/** Remove one member; the rest go back to plain lines at full price. */
export function breakGroup(lines: CartLine[], id: number, key: number): CartLine[] {
  return lines
    .filter((l) => l.key !== key)
    .map((l) => {
      if (l.group?.id !== id) return l
      const { group: _g, plugin: _p, ...rest } = l
      return { ...rest, discount: '', discountType: 'fixed' as const }
    })
}

/** Lines in cart order with each group's lines gathered under its first appearance. */
export function grouped(lines: CartLine[]): ({ kind: 'line'; line: CartLine } | { kind: 'group'; id: number; label: string; count: number; lines: CartLine[] })[] {
  const out: ReturnType<typeof grouped> = []
  const seen = new Map<number, CartLine[]>()
  for (const l of lines) {
    if (!l.group) {
      out.push({ kind: 'line', line: l })
      continue
    }
    const members = seen.get(l.group.id)
    if (members) {
      members.push(l)
      continue
    }
    const list = [l]
    seen.set(l.group.id, list)
    out.push({ kind: 'group', id: l.group.id, label: l.group.label, count: l.group.count, lines: list })
  }
  return out
}
