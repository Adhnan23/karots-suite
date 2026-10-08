import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Minus, Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ReturnRequest, ReturnResult, SaleLine, SaleView } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { money, num, qtyText, r2 } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, errMsg } from '@/features/till/api'
import { slipPath } from '@/features/till/print'
import { FIELD } from '@/features/till/ui'
import { SaleFind } from '../SaleFind'
import { afterSlip } from '../slip'
import { CounterPage } from '../ui'
import { left, REASONS, refundOf } from './refund'


/** Take goods back against a receipt: pick the lines, say if they're faulty, refund. */
export function ReturnsTab() {
  const cfg = useSuspenseQuery(configQuery).data
  const [id, setId] = useState<number | null>(null)
  const sale = useQuery({ queryKey: ['till', 'sale', id], queryFn: () => api<SaleView>(`/till/sales/${id}`), enabled: id !== null }).data
  return (
    <CounterPage flush aside={sale && sale.id === id ? <ReturnForm key={`${sale.id}-${sale.status}-${sale.lines.map((l) => l.returnedQty).join()}`} sale={sale} /> : (
      <p className="m-auto max-w-[280px] p-8 text-center text-faint">Scan or type the receipt number, or pick the sale from the list.</p>
    )}>
      <SaleFind title="Returns" sym={cfg.symbol} picked={id} onPick={setId} />
    </CounterPage>
  )
}

function ReturnForm({ sale }: { sale: SaleView }) {
  const cfg = useSuspenseQuery(configQuery).data
  const qc = useQueryClient()
  const sym = cfg.symbol
  const [qty, setQty] = useState<Record<number, number>>({})
  const [faulty, setFaulty] = useState<Record<number, boolean>>({})
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const value = r2(sale.lines.reduce((s, l) => s + refundOf(l, qty[l.id] ?? 0), 0))
  const offAccount = Math.min(value, num(sale.owedOnSale))
  const cash = r2(value - offAccount)
  const picked = sale.lines.filter((l) => (qty[l.id] ?? 0) > 0)
  const bump = (l: SaleLine, d: number) => setQty((q) => ({ ...q, [l.id]: Math.min(left(l), Math.max(0, (q[l.id] ?? 0) + d)) }))

  async function submit() {
    if (!picked.length) return void toast.error('Choose what is coming back')
    setBusy(true)
    try {
      const body: ReturnRequest = {
        reason: reason.trim(), deferDrawerKick: !cfg.askToPrint,
        lines: picked.map((l) => ({ saleItemId: l.id, quantity: String(qty[l.id]), disposition: faulty[l.id] ? 'damage' : 'restock' })),
      }
      const res = await api<ReturnResult>(`/till/sales/${sale.id}/return`, { json: body })
      const c = num(res.cashRefund)
      afterSlip(cfg, c > 0 ? `Give back ${money(sym, c)} — return recorded` : 'Return recorded', slipPath.refund(sale.id), c > 0)
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <header className="flex items-start justify-between gap-3 px-5 pb-3 pt-5">
        <div>
          <h2 className="font-display text-xl font-bold tabular-nums">{sale.receiptNo}</h2>
          <p className="text-[13px] text-muted-foreground">
            {new Date(sale.createdAt).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })} · {sale.customerName || 'Walk-in'} · {sale.cashierName}
          </p>
        </div>
        <div className="text-right">
          <b className="font-display text-lg tabular-nums">{money(sym, sale.total)}</b>
          <small className="block text-muted-foreground">{sale.payments.map((p) => p.method).join(' + ')}</small>
        </div>
      </header>
      <ul aria-label="Items on the receipt" className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-3">
        {sale.lines.map((l) => {
          const n = qty[l.id] ?? 0
          const can = !l.isService && left(l) > 0
          return (
            <li key={l.id} className={cn('rounded-[var(--radius-sm)] px-3 py-2', n > 0 ? 'bg-[color-mix(in_srgb,var(--brand)_12%,var(--card))] ring-1 ring-brand' : 'bg-muted', !can && 'opacity-50')}>
              <div className="flex items-center gap-3">
                <span className="min-w-0 flex-1">
                  <b className="block truncate text-[14px]">{l.name}</b>
                  <small className="text-muted-foreground tabular-nums">
                    {qtyText(l.quantity)} {l.unitAbbr} × {money(sym, l.refundUnit)}
                    {num(l.returnedQty) > 0 && ` · ${qtyText(l.returnedQty)} already back`}
                    {l.isService && ' · service, not returnable'}
                  </small>
                </span>
                {can && (
                  <div className="flex items-center gap-1" aria-label={`Return ${l.name}`}>
                    <button type="button" aria-label={`Fewer ${l.name}`} onClick={() => bump(l, -1)} className="grid h-8 w-8 place-items-center rounded-full border border-border bg-card"><Minus size={14} /></button>
                    <span className="w-14 text-center font-bold tabular-nums">{n}<small className="font-normal text-muted-foreground">/{qtyText(left(l))}</small></span>
                    <button type="button" aria-label={`More ${l.name}`} onClick={() => bump(l, 1)} className="grid h-8 w-8 place-items-center rounded-full border border-border bg-card"><Plus size={14} /></button>
                  </div>
                )}
              </div>
              {n > 0 && (
                <label className="mt-1.5 flex items-center gap-2 text-[12.5px] text-muted-foreground">
                  <input type="checkbox" className="accent-[var(--brand)]" checked={!!faulty[l.id]} onChange={(e) => setFaulty((f) => ({ ...f, [l.id]: e.target.checked }))} />
                  Faulty — don't put it back on the shelf (write it off)
                </label>
              )}
            </li>
          )
        })}
      </ul>
      <footer className="flex flex-col gap-3 border-t border-border p-4">
        <div className="flex flex-wrap items-center gap-1.5">
          {REASONS.map((r) => (
            <button key={r} type="button" aria-pressed={reason === r} onClick={() => setReason(reason === r ? '' : r)}
              className="rounded-full border border-border px-3 py-1 text-[12.5px] font-semibold aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground">{r}</button>
          ))}
          <input aria-label="Reason" placeholder="Other reason" value={REASONS.includes(reason) ? '' : reason} onChange={(e) => setReason(e.target.value)} className={cn(FIELD, 'h-8 min-w-[140px] flex-1 text-[13px]')} />
        </div>
        {offAccount > 0 && (
          <p className="flex justify-between text-[13px] text-muted-foreground">Comes off {sale.customerName}'s account <b className="tabular-nums text-foreground">{money(sym, offAccount)}</b></p>
        )}
        <Button size="lg" disabled={busy || !picked.length} onClick={() => void submit()} className="h-12 text-[15px]">
          {!picked.length ? 'Choose what is coming back' : cash > 0 ? `Refund ${money(sym, cash)} cash` : 'Record the return'}
        </Button>
      </footer>
    </div>
  )
}
