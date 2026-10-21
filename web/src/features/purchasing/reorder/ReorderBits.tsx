import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ReorderAutoResult, ReorderLevelRequest, ReorderRow } from '@/api/types.gen'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { reorderQuery, type ReorderFilter } from '../queries'
import { learning, reasonLine } from './reason'

const CHIP = 'rounded-full border border-border bg-muted px-2 py-px text-[11px] font-bold text-foreground'

/** The Auto / Fixed chip and why the level is what it is ("sells 10 a day · supplier every 7 days"). */
export function ReasonLine({ r, onUse }: { r: ReorderRow; onUse?: (r: ReorderRow) => void }) {
  if (!r.calc) return null
  const learnt = r.auto ? learning(r.calc) : ''
  return (
    <small className="flex flex-wrap items-center gap-x-1.5 gap-y-1 text-[13px] text-foreground">
      <span className={CHIP}>{r.auto ? `Auto${learnt ? ` · ${learnt}` : ''}` : 'Fixed'}</span>
      {r.looksOff ? <>sales suggest {r.calc.calcLevel} ({reasonLine(r.calc)})</> : reasonLine(r.calc)}
      {r.looksOff && onUse && (
        <button type="button" onClick={() => onUse(r)} className="min-h-9 rounded-full border border-brand bg-brand-soft px-3 text-xs font-bold">Use {r.calc.calcLevel}</button>
      )}
    </small>
  )
}

/** Sets Fixed levels to what sales suggest ("suggested") or switches them to Auto. */
export function useSetLevels() {
  const qc = useQueryClient()
  return async (ids: number[], use: 'suggested' | 'auto') => {
    try {
      const body: ReorderLevelRequest = { ids, use }
      const r = await api<ReorderAutoResult>('/admin/products/reorder-level', { json: body })
      toast.success(use === 'auto' ? `${r.switched} switched to Auto` : `${r.switched} reorder level${r.switched === 1 ? '' : 's'} updated`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
}

/** How many Fixed levels look off, for the chip (0 hides it). */
export function useLooksOffCount() {
  return useQuery(reorderQuery({ q: '', category: 0, supplier: 0, page: 1, off: true })).data?.total ?? 0
}

/** Fixed levels far from what sales suggest — low or not. Nothing changes until the owner picks. */
export function LooksOffList({ f }: { f: ReorderFilter }) {
  const rows = useQuery(reorderQuery({ ...f, page: 1, all: true, off: true })).data?.items
  const setLevels = useSetLevels()
  const [busy, setBusy] = useState(false)
  const run = async (ids: number[], use: 'suggested' | 'auto') => { setBusy(true); await setLevels(ids, use); setBusy(false) }
  if (!rows) return <section className={cn(SURFACE, 'mb-24')}><EmptyState title="Loading…" /></section>
  if (rows.length === 0) return <section className={cn(SURFACE, 'mb-24')}><EmptyState title="Every Fixed level is close to what sales need" /></section>
  const ids = rows.map((r) => r.productId)
  return (
    <div className="mb-24 flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">These products have a <b className="text-foreground">Fixed</b> level far from what their sales need. Nothing changes until you choose.</p>
      <section className={cn(SURFACE, 'overflow-hidden')}>
        <ul aria-label="Levels look off" className="divide-y divide-border">
          {rows.map((r) => (
            <li key={r.productId} className="flex items-center gap-3 px-4 py-3">
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <b className="truncate">{r.name}</b>
                <small className="text-[13px] text-muted-foreground">Fixed {r.reorderLevel} → sales suggest <b className="text-foreground">{r.calc?.calcLevel}</b></small>
                {r.calc && <small className="text-xs text-faint">{reasonLine(r.calc)}</small>}
              </span>
              <Button variant="outline" className="h-11 shrink-0 px-3 font-bold" disabled={busy} onClick={() => void run([r.productId], 'suggested')}>Use {r.calc?.calcLevel}</Button>
            </li>
          ))}
        </ul>
      </section>
      <div className="flex flex-col gap-2 md:flex-row">
        <Button className="h-12 font-bold md:px-6" disabled={busy} onClick={() => void run(ids, 'suggested')}>Use all {ids.length}</Button>
        <Button variant="outline" className="h-11 md:px-6" disabled={busy} onClick={() => void run(ids, 'auto')}>Switch these {ids.length} to Auto instead</Button>
      </div>
    </div>
  )
}
