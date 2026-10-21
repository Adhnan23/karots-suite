import { toast } from 'sonner'
import type { TillConfig } from '@/api/types.gen'
import { slipPath } from '@/features/till/print'
import { slipOut } from '@/features/till/SlipAsk'

/** After a counter action with a slip (its print path): in "ask before printing"
 * mode the Print / Skip prompt shows it as it will print; otherwise it prints
 * straight away, with the drawer pulse when cash left the drawer (the server
 * skipped its own kick for that case). */
export function afterSlip(_cfg: TillConfig, msg: string, slip: string | null, cashFromDrawer: boolean) {
  if (!slip) return void toast.success(msg)
  void slipOut(slip, msg, { kick: cashFromDrawer })
}

/** A CR- money slip (expense, supplier payment…). */
export const moneySlip = (id: number | undefined) => (id ? slipPath.money(id) : null)

/** Which Receipts list a slip number belongs to (a scanned S- / CR- / DP-
 * barcode), or null for anything else — such text is never a product barcode. */
export function slipListOf(text: string): 'sales' | 'cash' | 'credit' | null {
  const m = /^(S|CR|DP)-\d+$/i.exec(text.trim())
  if (!m) return null
  return ({ S: 'sales', CR: 'cash', DP: 'credit' } as const)[m[1].toUpperCase() as 'S' | 'CR' | 'DP']
}
