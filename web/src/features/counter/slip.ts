import { toast } from 'sonner'
import type { TillConfig } from '@/api/types.gen'
import { printMoneySlip } from '@/features/till/print'

/** After a counter cash move: in "ask before printing" mode the toast offers the
 * CR- slip; otherwise it prints straight away (with the drawer pulse when the cash
 * came from the drawer — the server skipped its own kick for that case). */
export function afterMove(cfg: TillConfig, msg: string, receiptId: number | undefined, fromDrawer: boolean) {
  if (!receiptId) return void toast.success(msg)
  if (cfg.askToPrint) {
    toast.success(msg, { duration: 10_000, action: { label: 'Print receipt', onClick: () => void printMoneySlip(receiptId) } })
    return
  }
  toast.success(msg)
  void printMoneySlip(receiptId, fromDrawer)
}
