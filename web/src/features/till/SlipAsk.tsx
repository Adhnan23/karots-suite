import { Printer } from 'lucide-react'
import { useSyncExternalStore } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { toast } from 'sonner'
import { queryClient } from '@/api/query-client'
import { configQuery, errMsg } from './api'
import { printSlip } from './print'
import { SlipPaper } from './SlipPaper'

/** "Ask before printing" (Settings): every slip the till or a counter tab makes
 * asks here — Print or Skip — with the receipt as it will print. One prompt for
 * drawer moves, refunds, credit, expenses, supplier pay and warranty swaps. */
type Ask = { path: string; title: string; note: string }

let ask: Ask | null = null
const subs = new Set<() => void>()
const set = (a: Ask | null) => { ask = a; subs.forEach((f) => f()) }
const subscribe = (f: () => void) => { subs.add(f); return () => void subs.delete(f) }

export function askToPrint(a: Ask) { set(a) }

/** A slip by Settings' rule, from anywhere (till, counter, admin, a plugin): "ask
 * before printing" → the Print / Skip prompt with the slip as it will print;
 * otherwise it prints straight away. msg says what happened. kick folds the drawer
 * pulse into the print (cash just left / came in at this till). */
export async function slipOut(path: string, msg: string, opts: { title?: string; kick?: boolean } = {}) {
  const cfg = await queryClient.ensureQueryData(configQuery).catch(() => null)
  if (cfg?.askToPrint) return askToPrint({ path, title: opts.title ?? 'Print the receipt?', note: msg })
  if (msg) toast.success(msg)
  await printSlip(path, opts.kick)
}

/** A reprint asked for by hand (a ledger row's Print): the same rule — the prompt
 * shows it first when the shop asks before printing. */
export const reprintSlip = (path: string, receiptNo: string) =>
  slipOut(path, `${receiptNo} — print it again?`, { title: `Reprint ${receiptNo}?` }).catch((e) => toast.error(errMsg(e)))

/** Mounted once in the till layout (Sell and every counter tab) and once in the admin layout. */
export function SlipAskHost() {
  const a = useSyncExternalStore(subscribe, () => ask)
  if (!a) return null
  const close = () => set(null)
  return (
    <Dialog open onOpenChange={(o) => !o && close()}>
      <DialogContent className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>{a.title}</DialogTitle>
          <DialogDescription>{a.note}</DialogDescription>
        </DialogHeader>
        <div className="max-h-[56vh] overflow-auto rounded-[var(--radius-sm)] bg-muted p-4">
          <SlipPaper key={a.path} path={a.path} />
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={close}>Skip</Button>
          <Button autoFocus onClick={() => { void printSlip(a.path); close() }}><Printer size={16} /> Print</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
