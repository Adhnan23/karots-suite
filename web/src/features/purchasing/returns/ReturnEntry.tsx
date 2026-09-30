import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { PurchaseReturnRequest, PurchaseReturnRow } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { suppliersQuery } from '@/features/counter/queries'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num, qtyText, r2 } from '@/lib/money'
import { cn } from '@/lib/utils'
import { day } from '../purchases/PurchasesPage'
import { purchaseQuery, purchasesQuery, returnableQuery } from '../queries'
import { useFormKeys } from '../useFormKeys'

/** Send goods back from one received delivery (an upgrade on karots-pos, where
 * any product could go to any supplier at any typed cost). Only that delivery's
 * lines can go back, at most what's left of them, at what they really cost —
 * after the line and bill discounts and free units — and the credit lands on
 * that delivery's bill first. */
export function ReturnEntry({ supplierId, purchaseId }: { supplierId: number; purchaseId: number }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const go = useNavigate()
  const sups = useQuery(suppliersQuery('')).data ?? []
  const fixed = useQuery({ ...purchaseQuery(purchaseId), enabled: purchaseId > 0 }).data
  const [sup, setSup] = useState(supplierId)
  const [pid, setPid] = useState(purchaseId)
  const supplier = fixed?.supplierId ?? sup
  const deliveries = useQuery({ ...purchasesQuery({ tab: 'received', q: '', supplier, unpaid: false, page: 1 }), enabled: supplier > 0 && !purchaseId }).data?.items ?? []
  const lines = useQuery({ ...returnableQuery(pid), enabled: pid > 0 }).data?.lines ?? []
  const [qty, setQty] = useState<Record<number, string>>({})
  const [reference, setReference] = useState('')
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const picked = lines.filter((l) => num(qty[l.batchId]) > 0)
  const total = r2(picked.reduce((t, l) => t + num(qty[l.batchId]) * num(l.cost), 0))
  const over = picked.some((l) => num(qty[l.batchId]) > num(l.left))

  async function save() {
    setBusy(true)
    try {
      const body: PurchaseReturnRequest = { supplierId: supplier, purchaseId: pid, reference, reason,
        lines: picked.map((l) => ({ productId: l.productId, quantity: qty[l.batchId], costPrice: '', batchId: l.batchId })) }
      const r = await api<PurchaseReturnRow>('/admin/purchase-returns', { json: body })
      toast.success(`Returned ${money(sym, r.total)} of stock to ${r.supplierName}`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      void go({ to: '/admin/purchase-returns/$id', params: { id: String(r.id) } })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  useFormKeys(null, () => { if (!busy && pid && total > 0 && !over) void save() })
  const label = (d: { invoiceNo: string; id: number; date: string; total: string }) => `${d.invoiceNo || `GRN #${d.id}`} · ${day(d.date)} · ${money(sym, d.total)}`

  return (
    <>
      <Link to="/admin/purchase-returns" className="mb-2 inline-flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft size={14} /> Purchase returns</Link>
      <h1 className="mb-4 font-display text-[24px] font-bold tracking-tight md:text-[28px]">Return goods{fixed && ` to ${fixed.supplierName}`}</h1>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <section className={cn(SURFACE, 'flex flex-col gap-3 p-4')}>
          {fixed ? (
            <p className="text-sm text-muted-foreground">From delivery <b className="text-foreground">{label(fixed)}</b></p>
          ) : (
            <div className="grid gap-2 md:grid-cols-2">
              <Field label="Supplier">
                <select aria-label="Supplier" value={sup} onChange={(e) => { setSup(Number(e.target.value)); setPid(0); setQty({}) }} className={cn(INPUT, 'appearance-auto')}>
                  <option value={0}>— pick a supplier —</option>
                  {sups.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
                </select>
              </Field>
              <Field label="Delivery it came in on">
                <select aria-label="Delivery" value={pid} disabled={!sup} onChange={(e) => { setPid(Number(e.target.value)); setQty({}) }} className={cn(INPUT, 'appearance-auto disabled:opacity-60')}>
                  <option value={0}>{sup && !deliveries.length ? 'Nothing received from them yet' : '— pick the delivery —'}</option>
                  {deliveries.map((d) => <option key={d.id} value={d.id}>{label(d)}</option>)}
                </select>
              </Field>
            </div>
          )}
          {pid > 0 && (
            <ul aria-label="Lines" className="flex flex-col gap-1.5">
              {lines.map((l) => {
                const q = qty[l.batchId] ?? ''
                const disc = num(l.cost) < num(l.listCost)
                return (
                  <li key={l.batchId} className={cn('grid grid-cols-[minmax(0,1fr)_88px] items-center gap-2 rounded-[var(--radius-sm)] bg-muted p-2.5', num(l.left) === 0 && 'opacity-60')}>
                    <span className="min-w-0">
                      <b className="block truncate text-sm">{l.name}</b>
                      <small className="block text-faint tabular-nums">
                        {qtyText(l.left)} of {qtyText(l.received)} {l.unitAbbr} left from this delivery · {money(sym, l.cost)} each
                        {disc && <> <s>{money(sym, l.listCost)}</s> after discounts</>}{l.expiry && ` · exp ${l.expiry}`}
                      </small>
                    </span>
                    <input aria-label={`Return how many ${l.name}`} inputMode="decimal" placeholder="0" value={q} disabled={num(l.left) === 0}
                      onChange={(e) => setQty({ ...qty, [l.batchId]: e.target.value.replace(/[^\d.]/g, '') })}
                      className={cn(INPUT, 'h-10 px-2 text-right font-bold tabular-nums', num(q) > num(l.left) && 'border-danger')} />
                  </li>
                )
              })}
              {lines.length === 0 && <li className="p-6 text-center text-sm text-faint">Loading the delivery…</li>}
            </ul>
          )}
          {!pid && !fixed && <p className="py-8 text-center text-sm text-faint">Pick the supplier and the delivery the goods came in on.</p>}
        </section>
        <aside className={cn(SURFACE, 'flex flex-col gap-3.5 self-start p-4')}>
          <Field label="Reference"><input aria-label="Reference" value={reference} onChange={(e) => setReference(e.target.value)} className={INPUT} placeholder="Debit note no (optional)" /></Field>
          <Field label="Reason"><input aria-label="Reason" value={reason} onChange={(e) => setReason(e.target.value)} className={INPUT} placeholder="Damaged, wrong item, expired…" /></Field>
          <p className="flex justify-between border-t border-border pt-3 text-base font-bold tabular-nums"><span>Value</span><span className="text-success">↓ {money(sym, total)}</span></p>
          <small className="-mt-2 text-faint">Comes off this delivery's bill first; anything more comes off their other bills or stays as credit.</small>
          {over && <small className="-mt-1 font-semibold text-danger">More than is left from this delivery.</small>}
          <Button size="lg" disabled={busy || !pid || !(total > 0) || over} onClick={() => void save()} className="h-12 font-bold">Return {money(sym, total)} of goods</Button>
        </aside>
      </div>
    </>
  )
}
