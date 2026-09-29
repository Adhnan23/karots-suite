import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { WarrantyReceiptLine, WarrantyReceiptReplaceRequest, WarrantyReceiptView } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { num, qtyText } from '@/lib/money'
import { warrantyReceiptQuery } from '../queries'
import { REASONS } from './reasons'
import { Cover } from './UnitClaim'

/** A receipt's warranted, un-serialled lines: replace one like-for-like. */
export function ReceiptClaim({ no }: { no: string }) {
  const q = useQuery(warrantyReceiptQuery(no))
  if (q.error) {
    const nf = q.error instanceof ApiError && q.error.status === 404
    return <p className="m-auto max-w-[280px] p-8 text-center text-faint">{nf ? `No sale with receipt “${no}”.` : errMsg(q.error)}</p>
  }
  const v = q.data
  if (!v) return null
  return (
    <div className="flex flex-col gap-4 overflow-auto p-5">
      <div>
        <h2 className="font-display text-xl font-bold tabular-nums">{v.receiptNo}</h2>
        <p className="text-[13px] text-muted-foreground">
          Sold {new Date(v.soldAt).toLocaleDateString([], { dateStyle: 'medium' })}{v.customerName && ` · ${v.customerName}`}
        </p>
      </div>
      {v.lines.length === 0 && <p className="text-faint">Nothing on this receipt carries a warranty.</p>}
      {v.lines.map((l) => <Line key={l.productId} v={v} l={l} />)}
    </div>
  )
}

function Line({ v, l }: { v: WarrantyReceiptView; l: WarrantyReceiptLine }) {
  const qc = useQueryClient()
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const left = num(l.remaining)

  async function replace() {
    setBusy(true)
    try {
      const body: WarrantyReceiptReplaceRequest = { saleId: v.saleId, productId: l.productId, reason }
      await api('/till/warranty/receipt-replace', { json: body })
      toast.success(`Replacement recorded — give the customer a new ${l.name}`)
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="flex flex-col gap-3 rounded-[var(--radius-sm)] border border-border p-4">
      <div className="flex items-baseline justify-between gap-3">
        <b>{l.name}</b>
        <small className="text-muted-foreground tabular-nums">{qtyText(l.qty)} {l.unitAbbr} sold · {l.replaced} replaced</small>
      </div>
      <Cover ok={l.underWarranty} until={l.until} left={l.underWarranty ? `${qtyText(left)} still claimable` : 'expired'} months={l.months} />
      {l.underWarranty && left > 0 && (
        <>
          <div className="flex flex-wrap gap-1.5">
            {REASONS.map((r) => (
              <button key={r} type="button" aria-pressed={reason === r} onClick={() => setReason(reason === r ? '' : r)}
                className="rounded-full border border-border px-3 py-1 text-[12.5px] font-semibold aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground">{r}</button>
            ))}
          </div>
          <Button disabled={busy} onClick={() => void replace()} className="h-11">Replace one {l.name}</Button>
        </>
      )}
    </section>
  )
}
