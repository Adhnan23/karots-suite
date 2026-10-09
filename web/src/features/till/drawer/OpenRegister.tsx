import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { DrawerState, ReceiptRef } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery, errMsg, holdsCash } from '../api'
import { useTillUi } from '../TillUi'
import { PANEL } from '../ui'
import { clearOpenDraft, fromBreakdown, loadOpenDraft, lockerDefault, saveOpenDraft, toBreakdown, type Counts } from './count'
import { DenomCounter } from './DenomCounter'
import { LockerSelect } from './LockerSelect'
import { useDrawerSections } from './sections'

/** Covers the sell screen until the drawer is counted and opened. */
export function OpenRegister({ state }: { state: DrawerState }) {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const qc = useQueryClient()
  const draft = useMemo(() => loadOpenDraft(), [])
  const lockers = state.lockers.filter(holdsCash)
  const [counts, setCounts] = useState<Counts>(draft?.counts ?? {})
  const [lockerId, setLockerId] = useState(draft?.lockerId ?? lockerDefault(lockers, cfg.defaultLockerId, 'in', true))
  const [busy, setBusy] = useState(false)
  const sections = useDrawerSections('open')
  const last = state.summary.lastClosing

  useEffect(() => {
    if (draft) toast.success('Recovered your opening count')
  }, [draft])
  useEffect(() => {
    if (Object.keys(counts).length) saveOpenDraft(counts, lockerId)
  }, [counts, lockerId])

  async function open() {
    setBusy(true)
    try {
      const res = await api<ReceiptRef>('/till/drawer/open', {
        json: { breakdown: toBreakdown(state.denominations, counts), sourceLockerId: num(lockerId) },
      })
      clearOpenDraft()
      await sections.runAll() // best-effort for an open: reconciliation carries the last close
      await qc.invalidateQueries({ queryKey: drawerQuery.queryKey })
      toast.success('Till opened')
      ui.afterMove(res.receiptId)
      document.getElementById('till-search')?.focus()
    } catch (e) {
      toast.error(errMsg(e, 'Could not open the till'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div role="dialog" aria-modal="true" aria-labelledby="open-till-title"
      className="absolute inset-0 z-20 grid place-items-center bg-[color-mix(in_srgb,var(--background)_72%,transparent)] p-6 backdrop-blur-sm">
      <motion.div initial={{ scale: 0.96, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} transition={{ duration: 0.22 }}
        className={cn(PANEL, 'w-full max-w-[760px] p-6')}>
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 id="open-till-title" className="font-display text-2xl font-bold">Open the till</h2>
            <p className="text-muted-foreground">Count the float in the drawer, then open.</p>
          </div>
          {last && cfg.allowContinueLast && (
            <div className="flex gap-2">
              <Button variant="secondary" onClick={() => setCounts(fromBreakdown(state.summary.lastBreakdown))}>
                Continue last count · {money(cfg.symbol, last)}
              </Button>
              <Button variant="ghost" onClick={() => setCounts({})}>Count fresh</Button>
            </div>
          )}
        </div>
        <div className={cn('mt-5 grid gap-6', sections.nodes.length > 0 && 'grid-cols-2')}>
          <div className="flex flex-col gap-4">
            <DenomCounter denoms={state.denominations} counts={counts} onChange={setCounts} sym={cfg.symbol} label="Opening float" />
            <LockerSelect label="Float taken from" lockers={lockers} value={lockerId} onChange={setLockerId}
              untrackedLabel="— already in the drawer —" sym={cfg.symbol} />
          </div>
          {sections.nodes.length > 0 && <div className="flex flex-col gap-4">{sections.nodes}</div>}
        </div>
        <Button className="mt-6 h-12 w-full text-base" disabled={busy} onClick={open}>Open the till</Button>
      </motion.div>
    </div>
  )
}
