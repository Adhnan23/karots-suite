import type { CartLine } from '@/features/till/cart/types'
import { lineDiscount, lineNet } from '@/features/till/cart/totals'
import type { TillApi } from '@/plugins/slots'
import { recordSold } from './api'
import { setLines } from './pricing'
import type { Combo } from './types'

/** The label a combo's lines carry in the cart and on the receipt. */
export const groupLabel = (c: Combo) => `${c.name} (combo)`

/** Ring `sets` of a combo, optionally taking units off plain lines already in the cart. */
export const addCombo = (till: TillApi, c: Combo, sets: number, take?: { key: number; qty: number }[]) =>
  till.addGroup({ label: groupLabel(c), count: sets, items: setLines(c, till.saleType), take, plugin: { combos: { comboId: c.id } } })

const comboOf = (l: CartLine) => (l.group ? Number((l.plugin?.combos as { comboId?: number } | undefined)?.comboId) || 0 : 0)

/** After a sale: what each combo sold (sets, net, discount) for the sold counts and the report. */
export function soldRows(lines: CartLine[]) {
  const by = new Map<number, { sets: number; net: number; discount: number; groups: Set<number> }>()
  for (const l of lines) {
    const id = comboOf(l)
    if (!id) continue
    const r = by.get(id) ?? { sets: 0, net: 0, discount: 0, groups: new Set<number>() }
    if (!r.groups.has(l.group!.id)) {
      r.groups.add(l.group!.id)
      r.sets += l.group!.count
    }
    r.net += lineNet(l)
    r.discount += lineDiscount(l)
    by.set(id, r)
  }
  return [...by].map(([comboId, r]) => ({ comboId, sets: String(r.sets), net: r.net.toFixed(2), discount: r.discount.toFixed(2) }))
}

export async function afterSale({ saleId, lines }: { saleId: number; lines: CartLine[] }) {
  const rows = soldRows(lines)
  if (rows.length) await recordSold(saleId, rows)
}
