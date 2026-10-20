import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { CloseResult, Denomination, Locker, TillSessionRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { countTotal, lockerDefault, toBreakdown, type Counts } from '@/features/till/drawer/count'
import { DenomCounter } from '@/features/till/drawer/DenomCounter'
import { LockerSelect } from '@/features/till/drawer/LockerSelect'
import { configQuery, drawerQuery, errMsg, holdsCash } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lockersQuery } from '../queries'

/** Close a cashier's till for them (admin / manager) — they went home without closing.
 * The same count and banking as the till's own close; the server books it as theirs
 * and notes who closed it. */
export function CloseForThem({ s, onClose }: { s: TillSessionRow; onClose: () => void }) {
  const denoms = useQuery(drawerQuery).data?.denominations
  // Any active cash locker, not only the cashier-reachable ones the till lists: the one
  // closing is an admin or manager (e.g. into the main safe).
  const lockers = useQuery(lockersQuery(false)).data?.items
  // The form starts once both are here: its locker default is picked from them.
  return denoms && lockers ? <CountForm s={s} denoms={denoms} lockers={lockers.filter((l) => l.active && holdsCash(l))} onClose={onClose} /> : null
}

function CountForm({ s, denoms, lockers, onClose }: { s: TillSessionRow; denoms: Denomination[]; lockers: Locker[]; onClose: () => void }) {
  const cfg = useSuspenseQuery(configQuery).data
  const qc = useQueryClient()
  const [counts, setCounts] = useState<Counts>({})
  const [lockerId, setLockerId] = useState(() => lockerDefault(lockers, cfg.defaultLockerId, 'out', cfg.allowUntrackedCash))
  const [result, setResult] = useState<CloseResult | null>(null)
  const [busy, setBusy] = useState(false)
  const counted = countTotal(denoms, counts)
  const noLocker = !cfg.allowUntrackedCash && !lockers.length

  async function submit() {
    if (!cfg.allowUntrackedCash && !num(lockerId) && counted > 0) {
      toast.error(lockers.length ? 'Choose a locker to bank the cash into' : 'No locker set up — add one first')
      return
    }
    setBusy(true)
    try {
      setResult(await api<CloseResult>(`/admin/tills/${s.id}/close`, { json: { breakdown: toBreakdown(denoms, counts), destLockerId: num(lockerId) } }))
      void qc.invalidateQueries({ queryKey: ['admin'] })
    } catch (e) {
      toast.error(errMsg(e, 'Could not close the till'))
    } finally {
      setBusy(false)
    }
  }

  const diff = num(result?.difference)
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-[700px]">
        {!result ? (
          <>
            <DialogHeader>
              <DialogTitle>Close {s.cashier}'s till for them</DialogTitle>
              <DialogDescription>Count what is in their drawer now. Expected in it: {money(cfg.symbol, s.expected)}. It's recorded as closed by you.</DialogDescription>
            </DialogHeader>
            <DenomCounter denoms={denoms} counts={counts} onChange={setCounts} sym={cfg.symbol} />
            <LockerSelect label="Bank cash into" lockers={lockers} value={lockerId} onChange={setLockerId}
              untrackedLabel={cfg.allowUntrackedCash ? '— leave cash in drawer —' : undefined} sym={cfg.symbol} />
            {noLocker && <p className="text-sm text-danger">No cash locker set up. Add one to bank the closing cash (untracked cash is turned off).</p>}
            <DialogFooter>
              <Button variant="ghost" onClick={onClose}>Cancel</Button>
              <Button disabled={busy || (noLocker && counted > 0)} onClick={submit}>Close &amp; reconcile</Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader><DialogTitle>{s.cashier}'s till closed</DialogTitle></DialogHeader>
            <dl className="grid grid-cols-[1fr_auto] gap-y-1.5 text-[15px] tabular-nums">
              <dt className="text-muted-foreground">Cash sales</dt><dd>{money(cfg.symbol, result.cashSales)}</dd>
              <dt className="text-muted-foreground">Expected</dt><dd>{money(cfg.symbol, result.expectedCash)}</dd>
              <dt className="text-muted-foreground">Counted</dt><dd>{money(cfg.symbol, result.counted)}</dd>
              <dt className="font-semibold">Over / short</dt>
              <dd className={cn('font-display text-xl font-bold', diff < 0 ? 'text-danger' : 'text-success')}>{diff > 0 ? '+' : ''}{money(cfg.symbol, diff)}</dd>
            </dl>
            <DialogFooter>
              <Button variant="secondary" asChild><a href={`/print/z/${result.sessionId}`} target="_blank" rel="noreferrer">Day-end (Z) report</a></Button>
              <Button autoFocus onClick={onClose}>Done</Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
