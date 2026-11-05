import type { ReorderRow } from '@/api/types.gen'

/** One stretch of the low-stock list: a set of rows a plugin says belong together, or a single row. */
export type RowBlock = { key: string; name: string; line: string; rows: ReorderRow[] }

/** Rows sharing a block sit together at the first one's place; every other row stands alone. */
export function blocks(rows: ReorderRow[]): RowBlock[] {
  const out: RowBlock[] = []
  const at = new Map<string, RowBlock>()
  for (const r of rows) {
    if (!r.block) {
      out.push({ key: `p${r.productId}`, name: '', line: '', rows: [r] })
      continue
    }
    let b = at.get(r.block)
    if (!b) {
      b = { key: `b${r.block}`, name: r.block, line: r.blockLine ?? '', rows: [] }
      at.set(r.block, b)
      out.push(b)
    }
    b.rows.push(r)
  }
  return out
}
