import type { SupplierValue } from './types'

/** The values matching a search (any part of the value or its supplier, any case), in the server's order. */
export function matchValues(rows: SupplierValue[], q: string): SupplierValue[] {
  const t = q.trim().toLowerCase()
  return t ? rows.filter((r) => r.value.toLowerCase().includes(t) || r.supplierName.toLowerCase().includes(t)) : rows
}
