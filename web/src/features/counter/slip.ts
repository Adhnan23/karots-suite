import { toast } from 'sonner'
import type { TillConfig } from '@/api/types.gen'
import { printMoneySlip } from '@/features/till/print'

/** After a counter action with a slip: in "ask before printing" mode the toast
 * offers it; otherwise it prints straight away, with the drawer pulse when cash
 * left the drawer (the server skipped its own kick for that case). */
export function afterSlip(cfg: TillConfig, msg: string, print: ((kick: boolean) => Promise<void>) | null, cashFromDrawer: boolean) {
  if (!print) return void toast.success(msg)
  if (cfg.askToPrint) {
    toast.success(msg, { duration: 10_000, action: { label: 'Print receipt', onClick: () => void print(false) } })
    return
  }
  toast.success(msg)
  void print(cashFromDrawer)
}

/** A CR- money slip (expense, supplier payment…). */
export const moneySlip = (id: number | undefined) => (id ? (kick: boolean) => printMoneySlip(id, kick) : null)
