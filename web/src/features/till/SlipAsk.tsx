import { Printer } from 'lucide-react'
import { useSyncExternalStore } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
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

/** Mounted once in the till layout (Sell and every counter tab). */
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
