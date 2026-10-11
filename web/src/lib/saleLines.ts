import type { SaleLine } from '@/api/types.gen'
import { num } from './money'

export type SaleLineBlock =
  | { kind: 'line'; line: SaleLine }
  | { kind: 'group'; no: number; label: string; sets: string; net: number; lines: SaleLine[] }

/** A sale's lines in order, each line group (lines the till added together)
 * gathered into one block where its first line stood. */
export function saleLineBlocks(lines: SaleLine[]): SaleLineBlock[] {
  const out: SaleLineBlock[] = []
  const at = new Map<number, Extract<SaleLineBlock, { kind: 'group' }>>()
  for (const l of lines) {
    if (!l.group) {
      out.push({ kind: 'line', line: l })
      continue
    }
    let g = at.get(l.group)
    if (!g) {
      g = { kind: 'group', no: l.group, label: l.groupLabel, sets: l.groupSets, net: 0, lines: [] }
      at.set(l.group, g)
      out.push(g)
    }
    g.lines.push(l)
    g.net += num(l.subtotal)
  }
  return out
}
