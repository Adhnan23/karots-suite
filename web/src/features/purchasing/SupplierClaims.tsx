import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, PackageX } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ConsumeResult, LossRow, SupplierClaims } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { RecoverSheet } from '@/features/reports/ops/RecoverSheet'
import { errMsg } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'

/** "From this supplier": their expired batches still on the shelf and the
 * write-offs still waiting on them, settled where their delivery is received
 * (replace — into the batch just received —, credit, or nothing). Hidden when
 * there is nothing. base '/till' = the counter's Suppliers tab. */
export function SupplierClaimsPanel({ supplierId, sym, base = '/admin', className }: { supplierId: number; sym: string; base?: '/admin' | '/till'; className?: string }) {
  const qc = useQueryClient()
  const key = [base.slice(1), 'supplier-claims', supplierId]
  const c = useQuery({ queryKey: key, queryFn: () => api<SupplierClaims>(`${base}/suppliers/${supplierId}/claims`), enabled: supplierId > 0 }).data
  const [open, setOpen] = useState(false)
  const [loss, setLoss] = useState<LossRow | null>(null)
  const [busy, setBusy] = useState(0)
  if (!c || (c.expired.length === 0 && c.waiting.length === 0)) return null
  async function writeOff(lotId: number, row: Omit<LossRow, 'sourceId' | 'id'>) {
    setBusy(lotId)
    try {
      const r = await api<ConsumeResult>(`${base}/suppliers/${supplierId}/claims/expired/${lotId}`, { method: 'POST' })
      void qc.invalidateQueries({ queryKey: [base.slice(1)] })
      setLoss({ ...row, sourceId: r.lossId, id: r.lossId })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(0)
    }
  }
  const n = c.expired.length + c.waiting.length
  const row = 'flex flex-wrap items-center gap-x-3 gap-y-1 rounded-[var(--radius-sm)] border border-border bg-card px-3 py-2 text-sm'
  return (
    <section aria-label="From this supplier" className={cn('rounded-[var(--radius)] border border-warning bg-[color-mix(in_srgb,var(--warning)_8%,transparent)] p-2.5', className)}>
      <button type="button" aria-expanded={open} onClick={() => setOpen(!open)} className="flex w-full items-center gap-2 text-left text-sm">
        <PackageX size={17} className="shrink-0 text-warning" />
        <span className="flex-1"><b>{n} to settle with them</b> — {[c.expired.length && `${c.expired.length} expired on the shelf`, c.waiting.length && `${c.waiting.length} written off`].filter(Boolean).join(' · ')}</span>
        <ChevronDown size={16} className={cn('transition-transform', open && 'rotate-180')} />
      </button>
      {open && (
        <ul className="mt-2 flex flex-col gap-1.5">
          {c.expired.map((l) => (
            <li key={`e${l.lotId}`} className={row}>
              <span className="min-w-0 flex-1"><b>{l.product}</b> <small className="text-faint">· {qtyText(l.qty)} {l.unit} expired {l.expiry} · {money(sym, num(l.qty) * num(l.cost))}</small></span>
              <Button type="button" size="sm" variant="outline" disabled={busy === l.lotId}
                onClick={() => void writeOff(l.lotId, { kind: 'damage', productId: l.productId, product: l.product, qty: l.qty, loss: String(num(l.qty) * num(l.cost)), note: '', by: '', at: '', outcome: '', recovered: '' })}>
                Write off & settle</Button>
            </li>
          ))}
          {c.waiting.map((w) => (
            <li key={`w${w.sourceId}`} className={row}>
              <span className="min-w-0 flex-1"><b>{w.product}</b> <small className="text-faint">· {qtyText(w.qty)} written off {new Date(w.at).toLocaleDateString([], { day: 'numeric', month: 'short' })} · {money(sym, w.loss)}{w.note && ` · ${w.note}`}</small></span>
              <Button type="button" size="sm" onClick={() => setLoss(w)}>Settle</Button>
            </li>
          ))}
        </ul>
      )}
      {loss && <RecoverSheet loss={loss} sym={sym} supplierId={supplierId} base={base} onClose={() => { setLoss(null); void qc.invalidateQueries({ queryKey: key }) }} />}
    </section>
  )
}
