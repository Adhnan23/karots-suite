import { Minus, Plus } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { SaleLine, SaleView } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { left, REASONS, refundOf } from '@/features/counter/returns/refund'
import { printRefund } from '@/features/till/print'
import { FIELD } from '@/features/till/ui'
import { dec, money, num, qtyText, r2 } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD } from '../../bits'
import { drawerOpen } from '../../drawer'
import { saleWithOverlay } from '../../overlay'
import { sendCall } from '../../sync'
import { AskPrint } from '../AskPrint'
import { CounterShell, type CounterProps } from './CounterShell'
import { loadSale, SaleFinder } from './SaleFinder'

/** Take goods back against a receipt, on or off the PC. Offline the money moves
 * now; the PC settles it when it hears. */
export function Returns(p: CounterProps) {
  const [id, setId] = useState<number | null>(null)
  const [sale, setSale] = useState<SaleView | null>(null)
  const [notice, setNotice] = useState('')
  useEffect(() => {
    if (id === null) return setSale(null)
    let stale = false
    void loadSale(id, p.live).then((s) => { if (!stale) setSale(s) })
    return () => { stale = true }
  }, [id, p.live])
  if (sale) return <ReturnForm key={sale.id} {...p} sale={saleWithOverlay(sale, p.out)} onDone={(n) => { setNotice(n); setId(null); p.onSent() }} onBack={() => setId(null)} />
  return (
    <CounterShell title="Returns" sub={p.live ? undefined : 'Sales of the last 30 days, as last synced'} onBack={p.onBack}>
      {notice && <p role="status" className="px-1 text-sm font-semibold text-success">{notice}</p>}
      <SaleFinder sym={p.copy.symbol} live={p.live} onPick={(i) => { setNotice(''); setId(i) }} />
    </CounterShell>
  )
}

function ReturnForm({ who, copy, out, live, sale, onDone, onBack }: CounterProps & { sale: SaleView; onDone: (notice: string) => void }) {
  const sym = copy.symbol
  const [qty, setQty] = useState<Record<number, number>>({})
  const [faulty, setFaulty] = useState<Record<number, boolean>>({})
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ask, setAsk] = useState<string | null>(null)
  const value = r2(sale.lines.reduce((s, l) => s + refundOf(l, qty[l.id] ?? 0), 0))
  const offAccount = Math.min(value, num(sale.owedOnSale))
  const cash = r2(value - offAccount)
  const picked = sale.lines.filter((l) => (qty[l.id] ?? 0) > 0)
  const open = drawerOpen(copy, who.id, out)
  const bump = (l: SaleLine, d: number) => setQty((q) => ({ ...q, [l.id]: Math.min(left(l), Math.max(0, (q[l.id] ?? 0) + d)) }))

  async function submit() {
    setBusy(true)
    setError('')
    const r = await sendCall(who.id, {
      method: 'POST', path: `/api/v1/till/sales/${sale.id}/return`, label: `Return on ${sale.receiptNo}`,
      cashOut: cash > 0 ? dec(cash) : undefined,
      body: { reason: reason.trim(), deferDrawerKick: true, lines: picked.map((l) => ({ saleItemId: l.id, quantity: String(qty[l.id]), disposition: faulty[l.id] ? 'damage' : 'restock' })) },
    })
    setBusy(false)
    if (r?.status === 'failed') return setError(r.note || 'The PC could not do this')
    const verb = cash > 0 ? `Give back ${money(sym, cash)}: return recorded` : 'Return recorded'
    if (!r) return onDone(`${verb} on this phone. It sends when the PC is back; no slip yet.`)
    const notice = `${verb}. The slip prints at the counter.`
    if (copy.askToPrint) return setAsk(notice)
    void printRefund(sale.id).catch(() => {})
    onDone(notice)
  }

  return (
    <CounterShell title={sale.receiptNo} sub={`${new Date(sale.createdAt).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })} · ${sale.customerName || 'Walk-in'} · ${money(sym, sale.total)}`} onBack={onBack}>
      <ul aria-label="Items on the receipt" className="flex flex-col gap-2">
        {sale.lines.map((l) => {
          const n = qty[l.id] ?? 0
          return (
            <li key={l.id} className={cn(CARD, 'px-3 py-2.5')}>
              <div className="flex items-center gap-2">
                <span className="min-w-0 flex-1">
                  <b className="block truncate text-[14.5px]">{l.name}</b>
                  <small className="text-muted-foreground tabular-nums">
                    {qtyText(l.quantity)} {l.unitAbbr} × {money(sym, l.refundUnit)}
                    {num(l.returnedQty) > 0 && ` · ${qtyText(l.returnedQty)} already back`}
                    {l.isService && ' · service: nothing goes back on a shelf'}
                  </small>
                </span>
                {left(l) > 0 && (
                  <div className="flex shrink-0 items-center gap-1" aria-label={`Return ${l.name}`}>
                    <button type="button" aria-label={`Fewer ${l.name}`} onClick={() => bump(l, -1)} className="grid h-11 w-11 place-items-center rounded-full bg-muted"><Minus size={16} /></button>
                    <span className="w-7 text-center font-bold tabular-nums">{n}</span>
                    <button type="button" aria-label={`More ${l.name}`} onClick={() => bump(l, 1)} className="grid h-11 w-11 place-items-center rounded-full bg-muted"><Plus size={16} /></button>
                  </div>
                )}
              </div>
              {n > 0 && !l.isService && (
                <label className="mt-2 flex min-h-8 items-center gap-2 text-[13px] text-muted-foreground">
                  <input type="checkbox" className="h-4 w-4 accent-[var(--brand)]" checked={!!faulty[l.id]} onChange={(e) => setFaulty((f) => ({ ...f, [l.id]: e.target.checked }))} />
                  Faulty: write it off, don't put it back
                </label>
              )}
            </li>
          )
        })}
      </ul>
      <div className="flex flex-wrap items-center gap-1.5">
        {REASONS.map((r) => (
          <button key={r} type="button" aria-pressed={reason === r} onClick={() => setReason(reason === r ? '' : r)}
            className="min-h-9 rounded-pill border border-border px-3 text-[13px] font-semibold aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground">{r}</button>
        ))}
      </div>
      <input aria-label="Reason" placeholder="Other reason" value={REASONS.includes(reason) ? '' : reason} onChange={(e) => setReason(e.target.value)} className={cn(FIELD, 'h-12 w-full')} />
      {offAccount > 0 && <p className="flex justify-between px-1 text-[13px] text-muted-foreground">Comes off {sale.customerName}'s account <b className="tabular-nums">{money(sym, offAccount)}</b></p>}
      {cash > 0 && !open && <p className="px-1 text-[13px] font-semibold text-warning">Open the drawer first: this is cash.</p>}
      {error && <p role="alert" className="px-1 text-[13px] font-semibold text-danger">{error}</p>}
      <Button size="lg" disabled={busy || !picked.length || (cash > 0 && !open)} onClick={() => void submit()} className="h-12 text-[15px]">
        {!picked.length ? 'Choose what is coming back' : cash > 0 ? `Refund ${money(sym, cash)} cash` : 'Record the return'}
      </Button>
      {ask && <AskPrint onPrint={() => void printRefund(sale.id).catch(() => {})} onClose={() => onDone(ask)} />}
    </CounterShell>
  )
}
