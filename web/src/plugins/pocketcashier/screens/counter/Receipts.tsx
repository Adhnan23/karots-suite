import { useEffect, useState } from 'react'
import { api } from '@/api/client'
import type { DebtReceiptRow, MoneyReceiptRow, SaleView, WarrantyClaimRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { printDebtSlip, printMoneySlip, printSale, printWarrantySlip } from '@/features/till/print'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD } from '../../bits'
import type { Action } from '../../store/outbox'
import type { CallInput } from '../../sync'
import { CounterShell, type CounterProps } from './CounterShell'
import { loadSale, SaleFinder } from './SaleFinder'

type Kind = 'sales' | 'cash' | 'credit' | 'warranty'
type Slip = { id: number; title: string; sub: string; print: () => Promise<unknown> }

const when = (iso: string) => new Date(iso).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

function waitingLabel(a: Action, sym: string): string {
  if (a.kind === 'call') return (a.body as CallInput).label
  if (a.kind === 'sale') return `Sale · ${money(sym, (a.body as { total?: string }).total ?? 0)}`
  return ''
}

/** Past slips. Sales are on the phone; a reprint and the other kinds of slip need the PC. */
export function Receipts(p: CounterProps) {
  const [kind, setKind] = useState<Kind>('sales')
  const [id, setId] = useState<number | null>(null)
  const [sale, setSale] = useState<SaleView | null>(null)
  const sym = p.copy.symbol
  useEffect(() => {
    if (id === null) return setSale(null)
    let stale = false
    void loadSale(id, p.live).then((s) => { if (!stale) setSale(s) })
    return () => { stale = true }
  }, [id, p.live])
  if (sale) return <SaleSlip sale={sale} sym={sym} live={p.live} onBack={() => setId(null)} />
  const waiting = p.out.filter((a) => a.userId === p.who.id).map((a) => ({ ref: a.ref, at: a.at, label: waitingLabel(a, sym) })).filter((a) => a.label)
  return (
    <CounterShell title="Receipts" onBack={p.onBack}>
      <Seg label="Receipt kind" value={kind} onChange={setKind} options={[['sales', 'Sales'], ['cash', 'Cash'], ['credit', 'Credit'], ['warranty', 'Warranty']]} />
      {kind === 'sales' && waiting.length > 0 && (
        <section aria-label="Waiting to send" className="flex flex-col gap-1">
          <h3 className="px-1 text-[12px] font-bold text-muted-foreground">Waiting to send</h3>
          {waiting.map((w) => <p key={w.ref} className={cn(CARD, 'px-4 py-2 text-[14px]')}>{w.label}</p>)}
        </section>
      )}
      {kind === 'sales' ? <SaleFinder sym={sym} live={p.live} onPick={setId} /> : <OtherSlips kind={kind} sym={sym} live={p.live} />}
    </CounterShell>
  )
}

function SaleSlip({ sale, sym, live, onBack }: { sale: SaleView; sym: string; live: boolean; onBack: () => void }) {
  return (
    <CounterShell title={sale.receiptNo} sub={`${when(sale.createdAt)} · ${sale.customerName || 'Walk-in'}`} onBack={onBack}>
      <ul className={cn(CARD, 'flex flex-col gap-1.5 px-4 py-3')}>
        {sale.lines.map((l) => (
          <li key={l.id} className="flex justify-between gap-3 text-[14px]">
            <span className="min-w-0">{l.name} <small className="text-muted-foreground tabular-nums">× {qtyText(l.quantity)} {l.unitAbbr}</small></span>
            <span className="shrink-0 tabular-nums">{money(sym, l.subtotal)}</span>
          </li>
        ))}
        <li className="mt-1 flex justify-between border-t border-border pt-2 font-bold"><span>Total</span><span className="tabular-nums">{money(sym, sale.total)}</span></li>
      </ul>
      {live ? (
        <div className="grid grid-cols-2 gap-2">
          <Button variant="outline" className="h-12" onClick={() => void printSale(sale.id, false, '80').catch(() => {})}>Print 80 mm</Button>
          <Button variant="outline" className="h-12" onClick={() => void printSale(sale.id, false, '58').catch(() => {})}>Print 58 mm</Button>
        </div>
      ) : <p className="px-1 text-[13px] text-muted-foreground">It prints when the PC is on.</p>}
    </CounterShell>
  )
}

async function slipsOf(kind: Exclude<Kind, 'sales'>, sym: string): Promise<Slip[]> {
  if (kind === 'cash') {
    const rows = await api<MoneyReceiptRow[]>('/till/money-receipts')
    return rows.map((r) => ({ id: r.id, title: r.receiptNo, sub: `${when(r.createdAt)} · ${r.kindLabel} · ${r.party} · ${money(sym, r.amount)}`, print: () => printMoneySlip(r.id) }))
  }
  if (kind === 'credit') {
    const rows = await api<DebtReceiptRow[]>('/till/customer-payments')
    return rows.map((r) => ({ id: r.id, title: r.receiptNo, sub: `${when(r.createdAt)} · ${r.customerName} · ${money(sym, r.amount)}`, print: () => printDebtSlip(r.id) }))
  }
  const rows = await api<WarrantyClaimRow[]>('/till/warranty-claims')
  return rows.map((r) => ({ id: r.id, title: r.oldSerial || `Claim ${r.id}`, sub: r.replacementSerial ? `→ ${r.replacementSerial}` : '', print: () => printWarrantySlip(r.id) }))
}

function OtherSlips({ kind, sym, live }: { kind: Exclude<Kind, 'sales'>; sym: string; live: boolean }) {
  const [rows, setRows] = useState<Slip[] | null>(null)
  const [printed, setPrinted] = useState('')
  useEffect(() => {
    if (!live) return
    let stale = false
    setRows(null)
    void slipsOf(kind, sym).then((r) => { if (!stale) setRows(r) }, () => { if (!stale) setRows([]) })
    return () => { stale = true }
  }, [kind, sym, live])
  if (!live) return <p className="m-auto p-8 text-center text-faint">These slips need the PC. Sales are on this phone.</p>
  return (
    <>
      <p className="px-1 text-[12.5px] text-muted-foreground">Tap a slip to print it at the counter.</p>
      {printed && <p role="status" className="px-1 text-sm font-semibold text-success">{printed} sent to the printer</p>}
      {rows?.length === 0 && <p className="m-auto p-8 text-center text-faint">No slips.</p>}
      {rows?.map((r) => (
        <button key={r.id} type="button" onClick={() => void r.print().then(() => setPrinted(r.title), () => {})} className={cn(CARD, 'flex min-h-14 flex-col justify-center px-4 py-2 text-left')}>
          <b className="truncate text-[15px] tabular-nums">{r.title}</b>
          <small className="truncate text-muted-foreground">{r.sub}</small>
        </button>
      ))}
    </>
  )
}
