import type { CartLine } from '@/features/till/cart/types'
import { qtyText } from '@/lib/money'
import type { Group, Tier } from './types'

/** A group shows tiers once one is named or there are two; else it's a plain list. */
export const hasTiers = (g: Group) => g.tiers.length > 1 || (g.tiers[0]?.name ?? '') !== ''

export const tierStatus = (t: Tier) => (t.reorderLevel <= 0 ? 'Not tracked' : t.low ? 'Low' : 'OK')

/** A plain cart line holding more than is in stock — the swap chip's cue. */
export const isShort = (l: CartLine) => !l.group && !l.isService && l.qty > l.stock

/** The group list as CSV: one row per tier. */
export function groupsCsv(gs: Group[]) {
  const q = (s: string) => `"${s.replace(/"/g, '""')}"`
  const rows = gs.flatMap((g) => g.tiers.map((t) => [q(g.name), q(t.name), qtyText(t.total), String(t.reorderLevel), tierStatus(t)]))
  return [['Group', 'Tier', 'In stock', 'Reorder at', 'Status'], ...rows].map((r) => r.join(',')).join('\n')
}
