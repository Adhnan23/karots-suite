import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { ExternalLink, Minus, Plus } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminReturnRequest, AdminReturnResult, SaleLine, SaleView } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { saleQuery } from '@/features/counter/queries'
import { SlipView } from '@/features/print/SlipView'
import { left, REASONS, refundOf } from '@/features/counter/returns/refund'
import { MoneySourcePicker } from '@/features/purchasing/MoneySource'
import { moneyDone } from '@/features/purchasing/suppliers/SupplierMoney'
import { Seg } from '@/features/till/bits'
import { configQuery, errMsg } from '@/features/till/api'
import { printSale, slipPath } from '@/features/till/print'
import { money, num, qtyText, r2 } from '@/lib/money'
import { cn } from '@/lib/utils'

/** One sale from the admin: the receipt as printed (reprint, A4) or a return
 * (karots-pos admin SaleReturnForm) with the cash refund paid from a picked
 * locker or open till. */
export function SaleSheet({ id, onClose }: { id: number; onClose: () => void }) {
  const sale = useQuery(saleQuery(id)).data
  const [mode, setMode] = useState<'receipt' | 'return'>('receipt')
  if (!sale) return null
  const canReturn = sale.lines.some((l) => left(l) > 0)
  const head = canReturn && <Seg label="View" value={mode} onChange={setMode} options={[['receipt', 'Receipt'], ['return', 'Return goods']]} />
  return mode === 'return' ? <ReturnSheet sale={sale} head={head} onClose={onClose} /> : <ReceiptView sale={sale} head={head} onClose={onClose} />
}

function ReceiptView({ sale, head, onClose }: { sale: SaleView; head: ReactNode; onClose: () => void }) {
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={sale.receiptNo} submit="Print again"
      onSubmit={() => void printSale(sale.id).then(() => toast.success('Sent to the printer'), (e) => toast.error(errMsg(e)))}
      extra={<Button variant="ghost" asChild className="h-10 px-3"><a href={`/print/receipt/${sale.id}`} target="_blank" rel="noreferrer"><ExternalLink size={15} /> A4 / PDF</a></Button>}>
      {head}
      <div className="rounded-[var(--radius-sm)] bg-muted/60 p-4"><SlipView path={slipPath.sale(sale.id)} /></div>
    </FormSheet>
  )
}

function ReturnSheet({ sale, head, onClose }: { sale: SaleView; head: ReactNode; onClose: () => void }) {
  const cfg = useSuspenseQuery(configQuery).data
  const qc = useQueryClient()
  const sym = cfg.symbol
  const [qty, setQty] = useState<Record<number, number>>({})
  const [faulty, setFaulty] = useState<Record<number, boolean>>({})
  const [reason, setReason] = useState('')
  const [source, setSource] = useState('')
  const [busy, setBusy] = useState(false)
  const lines = sale.lines.filter((l) => left(l) > 0)
  const value = r2(lines.reduce((s, l) => s + refundOf(l, qty[l.id] ?? 0), 0))
  const offAccount = Math.min(value, num(sale.owedOnSale))
  const cash = r2(value - offAccount)
  const picked = lines.filter((l) => (qty[l.id] ?? 0) > 0)
  const bump = (l: SaleLine, d: number) => setQty((q) => ({ ...q, [l.id]: Math.min(left(l), Math.max(0, (q[l.id] ?? 0) + d)) }))

  async function submit() {
    if (busy) return
    setBusy(true)
    try {
      const body: AdminReturnRequest = { reason: reason.trim(), source, lines: picked.map((l) => ({ saleItemId: l.id, quantity: String(qty[l.id]), disposition: faulty[l.id] ? 'damage' : 'restock' })) }
      const res = await api<AdminReturnResult>(`/admin/sales/${sale.id}/return`, { json: body })
      moneyDone(num(res.cashRefund) > 0 ? `Returned — ${money(sym, res.cashRefund)} refunded` : 'Return recorded', res)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Return on ${sale.receiptNo}`} wide
      submit={!picked.length ? 'Choose what comes back' : cash > 0 ? `↑ Refund ${money(sym, cash)}` : 'Record the return'}
      busy={busy || !picked.length || (cash > 0 && !source)} onSubmit={() => void submit()}
      extra={<Button type="button" variant="ghost" className="h-10 px-3" onClick={() => setQty(Object.fromEntries(lines.map((l) => [l.id, left(l)])))}>Return everything</Button>}>
      {head}
      <p className="text-[13px] text-muted-foreground">{sale.customerName || 'Walk-in'} · {money(sym, sale.total)} · {sale.payments.map((p) => p.method).join(' + ')}</p>
      <ul aria-label="Items on the receipt" className="flex flex-col gap-1">
        {lines.map((l) => {
          const n = qty[l.id] ?? 0
          return (
            <li key={l.id} className={cn('rounded-[var(--radius-sm)] px-3 py-2', n > 0 ? 'bg-[color-mix(in_srgb,var(--brand)_12%,var(--card))] ring-1 ring-brand' : 'bg-muted')}>
              <div className="flex items-center gap-3">
                <span className="min-w-0 flex-1">
                  <b className="block truncate text-[14px]">{l.name}</b>
                  {l.groupLabel && <span className="mr-1.5 inline-block rounded-md bg-brand-soft px-1.5 text-[11px] font-bold">{l.groupLabel}</span>}
                  <small className="text-muted-foreground tabular-nums">
                    sold {qtyText(l.quantity)} {l.unitAbbr} · {qtyText(left(l))} can come back · {money(sym, l.refundUnit)} each
                  </small>
                </span>
                <div className="flex items-center gap-1" aria-label={`Return ${l.name}`}>
                  <button type="button" aria-label={`Fewer ${l.name}`} onClick={() => bump(l, -1)} className="grid size-8 place-items-center rounded-full border border-border bg-card"><Minus size={14} /></button>
                  <span className="w-10 text-center font-bold tabular-nums">{n}</span>
                  <button type="button" aria-label={`More ${l.name}`} onClick={() => bump(l, 1)} className="grid size-8 place-items-center rounded-full border border-border bg-card"><Plus size={14} /></button>
                </div>
              </div>
              {n > 0 && !l.isService && (
                <label className="mt-1.5 flex items-center gap-2 text-[12.5px] text-muted-foreground">
                  <input type="checkbox" className="accent-[var(--brand)]" checked={!!faulty[l.id]} onChange={(e) => setFaulty((f) => ({ ...f, [l.id]: e.target.checked }))} />
                  Faulty — write it off instead of back to stock
                </label>
              )}
            </li>
          )
        })}
      </ul>
      <div className="flex flex-wrap items-center gap-1.5">
        {REASONS.map((r) => (
          <button key={r} type="button" aria-pressed={reason === r} onClick={() => setReason(reason === r ? '' : r)}
            className="rounded-full border border-border bg-muted px-3 py-1.5 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">{r}</button>
        ))}
        <input aria-label="Reason" placeholder="Other reason (optional)" value={REASONS.includes(reason) ? '' : reason} onChange={(e) => setReason(e.target.value)} className={cn(INPUT, 'h-9 min-w-[160px] flex-1 text-[13px]')} />
      </div>
      {value > 0 && (
        <div className="flex flex-col gap-1 rounded-[var(--radius-sm)] bg-muted px-3 py-2.5 text-sm">
          <span className="flex justify-between"><span className="text-muted-foreground">Worth (what was paid)</span><b className="tabular-nums">{money(sym, value)}</b></span>
          {offAccount > 0 && <span className="flex justify-between"><span className="text-muted-foreground">Comes off {sale.customerName}'s account</span><b className="tabular-nums">{money(sym, offAccount)}</b></span>}
          {cash > 0 && <span className="flex justify-between"><span className="text-muted-foreground">↑ Cash back</span><b className="tabular-nums text-danger">{money(sym, cash)}</b></span>}
        </div>
      )}
      {cash > 0 && (
        <Field label="↑ Refund cash from">
          <MoneySourcePicker method="cash" value={source} onChange={setSource} sym={sym} label="Refund cash from" />
        </Field>
      )}
    </FormSheet>
  )
}
