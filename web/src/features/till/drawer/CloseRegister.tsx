import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { CloseResult } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery, errMsg, holdsCash } from '../api'
import { printMoneySlip } from '../print'
import { useTillUi } from '../TillUi'
import { countTotal, lockerDefault, toBreakdown, type Counts } from './count'
import { DenomCounter } from './DenomCounter'
import { LockerSelect } from './LockerSelect'
import { useDrawerSections } from './sections'

export function CloseRegister({ logout }: { logout?: boolean }) {
  const cfg = useSuspenseQuery(configQuery).data
  const d = useQuery(drawerQuery).data
  const ui = useTillUi()
  const qc = useQueryClient()
  const lockers = (d?.lockers ?? []).filter(holdsCash)
  const denoms = d?.denominations ?? []
  const [counts, setCounts] = useState<Counts>({})
  const [lockerId, setLockerId] = useState(() => lockerDefault(lockers, cfg.defaultLockerId, 'out', cfg.allowUntrackedCash))
  const [result, setResult] = useState<CloseResult | null>(null)
  const [busy, setBusy] = useState(false)
  const sections = useDrawerSections('close')
  const counted = countTotal(denoms, counts)
  const noLocker = !cfg.allowUntrackedCash && !lockers.length

  async function submit() {
    if (!cfg.allowUntrackedCash && !num(lockerId) && counted > 0) {
      toast.error(lockers.length ? 'Choose a locker to bank the cash into' : 'No locker set up — ask an admin to add one first')
      return
    }
    if (!(await sections.runAll())) return
    setBusy(true)
    try {
      const r = await api<CloseResult>('/till/drawer/close', { json: { breakdown: toBreakdown(denoms, counts), destLockerId: num(lockerId) } })
      setResult(r)
      void qc.invalidateQueries({ queryKey: drawerQuery.queryKey })
      if (!cfg.askToPrint && r.receiptId) void printMoneySlip(r.receiptId)
    } catch (e) {
      toast.error(errMsg(e, 'Could not close the till'))
    } finally {
      setBusy(false)
    }
  }

  async function done() {
    ui.close()
    if (logout) {
      await api('/auth/logout', { method: 'POST' }).catch(() => undefined)
      window.location.assign('/login')
    }
  }

  const diff = num(result?.difference)
  return (
    <Dialog open onOpenChange={(o) => !o && (result ? void done() : ui.close())}>
      <DialogContent className="sm:max-w-[700px]">
        {!result ? (
          <>
            {logout && (
              <p className="rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] px-3 py-2 text-sm font-semibold text-warning">
                Close your till before signing out — count the drawer to finish.
              </p>
            )}
            <DialogHeader>
              <DialogTitle>Close the till</DialogTitle>
              <DialogDescription>Count the drawer to close this session. Expected in the drawer: {money(cfg.symbol, d?.summary.expected)}.</DialogDescription>
            </DialogHeader>
            <div className={cn('grid gap-6', sections.nodes.length > 0 && 'grid-cols-2')}>
              <div className="flex flex-col gap-4">
                <DenomCounter denoms={denoms} counts={counts} onChange={setCounts} sym={cfg.symbol} />
                <LockerSelect label="Bank cash into" lockers={lockers} value={lockerId} onChange={setLockerId}
                  untrackedLabel={cfg.allowUntrackedCash ? '— leave cash in drawer —' : undefined} sym={cfg.symbol} />
                {noLocker && <p className="text-sm text-danger">No cash locker set up. Ask an admin to add one to bank the closing cash (untracked cash is turned off).</p>}
              </div>
              {sections.nodes.length > 0 && <div className="flex flex-col gap-4">{sections.nodes}</div>}
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={ui.close}>Cancel</Button>
              <Button disabled={busy || (noLocker && counted > 0)} onClick={submit}>Close &amp; reconcile</Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader><DialogTitle>Till closed</DialogTitle></DialogHeader>
            <dl className="grid grid-cols-[1fr_auto] gap-y-1.5 text-[15px] tabular-nums">
              <dt className="text-muted-foreground">Cash sales</dt><dd>{money(cfg.symbol, result.cashSales)}</dd>
              <dt className="text-muted-foreground">Expected</dt><dd>{money(cfg.symbol, result.expectedCash)}</dd>
              <dt className="text-muted-foreground">Counted</dt><dd>{money(cfg.symbol, result.counted)}</dd>
              <dt className="font-semibold">Over / short</dt>
              <dd className={cn('font-display text-xl font-bold', diff < 0 ? 'text-danger' : 'text-success')}>{diff > 0 ? '+' : ''}{money(cfg.symbol, diff)}</dd>
            </dl>
            <DialogFooter>
              <Button variant="secondary" asChild><a href={`/print/z/${result.sessionId}`} target="_blank" rel="noreferrer">Day-end (Z) report</a></Button>
              {cfg.askToPrint && result.receiptId && <Button variant="secondary" onClick={() => void printMoneySlip(result.receiptId!)}>Print slip</Button>}
              <Button autoFocus onClick={() => void done()}>{logout ? 'Sign out' : 'Done'}</Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
