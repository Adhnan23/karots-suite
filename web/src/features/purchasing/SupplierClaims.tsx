import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, PackageX } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ClaimLot, LossRow, SupplierClaims } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { RecoverSheet } from '@/features/reports/ops/RecoverSheet'
import { errMsg } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { useGiveBack } from './GiveBackSheet'
import { useRecall } from './RecallPrompt'

const shortDay = (d: string) => new Date(d).toLocaleDateString([], { day: 'numeric', month: 'short' })

/** "From this supplier": their expired and close-to-expiry batches still on the
 * shelf, and what's been written off or given back and is waiting on them. A lot
 * is counted and given back; a waiting row is settled (replace — into the batch
 * just received —, credit, or nothing). Hidden when there is nothing. base
 * '/till' = the counter's Suppliers tab. */
export function SupplierClaimsPanel({ supplierId, sym, base = '/admin', className }: { supplierId: number; sym: string; base?: '/admin' | '/till'; className?: string }) {
  const qc = useQueryClient()
  const key = [base.slice(1), 'supplier-claims', supplierId]
  const c = useQuery({ queryKey: key, queryFn: () => api<SupplierClaims>(`${base}/suppliers/${supplierId}/claims`), enabled: supplierId > 0 }).data
  const [open, setOpen] = useState(false)
  const [loss, setLoss] = useState<LossRow | null>(null)
  const [busy, setBusy] = useState(0)
  const back = useGiveBack(sym, base)
  const recall = useRecall(base)
  if (!c || (c.recalled.length === 0 && c.expired.length === 0 && c.expiring.length === 0 && c.waiting.length === 0)) return <>{back.sheet}{recall.sheet}</>
  async function told(lotId: number) {
    setBusy(lotId)
    try {
      await api(`${base}/suppliers/${supplierId}/claims/lots/${lotId}/told`, { method: 'POST' })
      void qc.invalidateQueries({ queryKey: [base.slice(1)] })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(0)
    }
  }
  const n = c.recalled.length + c.expired.length + c.expiring.length + c.waiting.length
  const row = 'flex flex-wrap items-center gap-x-3 gap-y-1 rounded-[var(--radius-sm)] border border-border bg-card px-3 py-2 text-sm'
  const lot = (l: ClaimLot, status: string) => (
    <li key={`l${l.lotId}`} className={row}>
      <span className="min-w-0 flex-1 basis-full sm:basis-0"><b>{l.product}</b> <small className="text-faint">· {qtyText(l.qty)} {l.unit} {l.recalledOn ? `recalled ${shortDay(l.recalledOn)}, ${l.recallStops ? 'not selling' : 'still selling'}` : `${status} ${l.expiry}`} · {money(sym, num(l.qty) * num(l.cost))}{l.toldOn && ` · told ${shortDay(l.toldOn)}`}</small></span>
      <Button type="button" size="sm" variant="ghost" disabled={busy === l.lotId} onClick={() => void told(l.lotId)}>Told them</Button>
      {l.recalledOn
        ? <Button type="button" size="sm" variant="ghost" onClick={() => void recall.undo({ lotId: l.lotId, product: l.product, supplierId })}>Undo recall</Button>
        : <Button type="button" size="sm" variant="ghost" onClick={() => recall.ask({ lotId: l.lotId, product: l.product, supplierId })}>Recall</Button>}
      <Button type="button" size="sm" variant="outline"
        onClick={() => back.give({ lotId: l.lotId, productId: l.productId, product: l.product, unit: l.unit, qty: l.qty, expiry: l.expiry, supplierId, supplier: l.supplier })}>Give back</Button>
    </li>
  )
  return (
    <section aria-label="From this supplier" className={cn('rounded-[var(--radius)] border border-warning bg-[color-mix(in_srgb,var(--warning)_8%,transparent)] p-2.5', className)}>
      <button type="button" aria-expanded={open} onClick={() => setOpen(!open)} className="flex w-full items-center gap-2 text-left text-sm">
        <PackageX size={17} className="shrink-0 text-warning" />
        <span className="flex-1"><b>{n} to settle with them</b> — {[c.recalled.length && `${c.recalled.length} recalled`, c.expired.length && `${c.expired.length} expired on the shelf`, c.expiring.length && `${c.expiring.length} close to expiry`, c.waiting.length && `${c.waiting.length} waiting on them`].filter(Boolean).join(' · ')}</span>
        <ChevronDown size={16} className={cn('transition-transform', open && 'rotate-180')} />
      </button>
      {open && (
        <ul className="mt-2 flex flex-col gap-1.5">
          {c.recalled.map((l) => lot(l, ''))}
          {c.expired.map((l) => lot(l, 'expired'))}
          {c.expiring.map((l) => lot(l, 'expires'))}
          {c.waiting.map((w) => (
            <li key={`w${w.sourceId}`} className={row}>
              <span className="min-w-0 flex-1"><b>{w.product}</b> <small className="text-faint">· {qtyText(w.qty)} {w.givenBack ? 'given back' : 'written off'} {shortDay(w.at)} · {money(sym, w.loss)}</small></span>
              <Button type="button" size="sm" onClick={() => setLoss(w)}>Settle</Button>
            </li>
          ))}
        </ul>
      )}
      {back.sheet}
      {recall.sheet}
      {loss && <RecoverSheet loss={loss} sym={sym} supplierId={supplierId} base={base} onClose={() => { setLoss(null); void qc.invalidateQueries({ queryKey: key }) }} />}
    </section>
  )
}
