import { toast } from 'sonner'
import type { TillConfig } from '@/api/types.gen'
import { printSlip, slipPath } from '@/features/till/print'
import { askToPrint } from '@/features/till/SlipAsk'

/** After a counter action with a slip (its print path): in "ask before printing"
 * mode the Print / Skip prompt shows it as it will print; otherwise it prints
 * straight away, with the drawer pulse when cash left the drawer (the server
 * skipped its own kick for that case). */
export function afterSlip(cfg: TillConfig, msg: string, slip: string | null, cashFromDrawer: boolean) {
  if (!slip) return void toast.success(msg)
  if (cfg.askToPrint) return askToPrint({ path: slip, title: 'Print the receipt?', note: msg })
  toast.success(msg)
  void printSlip(slip, cashFromDrawer)
}

/** A CR- money slip (expense, supplier payment…). */
export const moneySlip = (id: number | undefined) => (id ? slipPath.money(id) : null)
