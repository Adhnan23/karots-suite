import type { ReorderRow, StockLine } from '@/api/types.gen'
import { num } from '@/lib/money'

// Low stock → Suppliers: "Order these" hands the lines to that supplier's Order
// job through sessionStorage (too long for a URL); the form clears it once read.
const KEY = 'ks-order-prefill'

/** Order lines for low-stock rows: the suggested quantity at the last cost; nothing to order → left out. */
export function linesFor(rows: ReorderRow[]): StockLine[] {
  return rows.filter((r) => num(r.suggested) > 0).map((r) => ({
    productId: r.productId, name: r.name, ordered: '', quantity: r.suggested, freeQty: '', costPrice: r.cost,
    sellingPrice: '', expiryDate: '', discount: '', discountType: 'fixed', packSize: '',
  }))
}

export function savePrefill(supplierId: number, lines: StockLine[]) {
  try { sessionStorage.setItem(KEY, JSON.stringify({ supplierId, lines })) } catch { /* private window: the order starts empty */ }
}

/** The lines saved for this supplier (read in a state initializer, so no side effect — see clearPrefill). */
export function peekPrefill(supplierId: number): StockLine[] | undefined {
  try {
    const p = JSON.parse(sessionStorage.getItem(KEY) ?? 'null') as { supplierId: number; lines: StockLine[] } | null
    return p?.supplierId === supplierId ? p.lines : undefined
  } catch {
    return undefined
  }
}

/** Forget the hand-over once the order form has it. */
export function clearPrefill() {
  try { sessionStorage.removeItem(KEY) } catch { /* nothing to clear */ }
}
