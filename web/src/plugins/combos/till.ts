import type { TillApi } from '@/plugins/slots'
import { setLines } from './pricing'
import type { Combo } from './types'

/** The label a combo's lines carry in the cart and on the receipt. */
export const groupLabel = (c: Combo) => `${c.name} (combo)`

/** Ring `sets` of a combo, optionally taking units off plain lines already in the cart. Each line carries
 * the combo's id: the server writes what the combo sold with the sale itself (plugins/combos sold.go). */
export const addCombo = (till: TillApi, c: Combo, sets: number, take?: { key: number; qty: number }[]) =>
  till.addGroup({ label: groupLabel(c), count: sets, items: setLines(c, till.saleType), take, plugin: { combos: { comboId: c.id } } })
