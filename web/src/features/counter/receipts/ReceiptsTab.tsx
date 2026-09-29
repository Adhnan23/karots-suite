import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { ExternalLink, Printer, Search } from 'lucide-react'
import { type ReactNode, useDeferredValue, useState } from 'react'
import type { DebtReceiptRow, MoneyReceiptRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { configQuery, drawerQuery } from '@/features/till/api'
import { printDebtSlip, printMoneySlip, printSale } from '@/features/till/print'
import { FIELD } from '@/features/till/ui'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { debtReceiptsQuery, moneyReceiptsQuery, saleQuery } from '../queries'
import { SaleFind } from '../SaleFind'
import { CounterPage, PaneTitle } from '../ui'
import { DebtPaper, MoneyPaper, SalePaper } from './Paper'

type Kind = 'sales' | 'cash' | 'credit'
type Pick = { kind: 'sales'; id: number } | { kind: 'cash'; row: MoneyReceiptRow } | { kind: 'credit'; row: DebtReceiptRow }

const when = (iso: string) => new Date(iso).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

/** Every slip the till has printed: find it, see it as paper, print it again. */
export function ReceiptsTab() {
  const cfg = useSuspenseQuery(configQuery).data
  const session = useQuery(drawerQuery).data?.summary.session
  const [kind, setKind] = useState<Kind>('sales')
  const [pick, setPick] = useState<Pick | null>(null)
  const sym = cfg.symbol
  const kinds = <Seg label="Receipt kind" value={kind} onChange={(k) => { setKind(k); setPick(null) }} options={[['sales', 'Sales'], ['cash', 'Cash'], ['credit', 'Credit']]} />
  return (
    <CounterPage flush aside={<Preview pick={pick} />}>
      {kind === 'sales' ? (
        <>
          <div className="flex items-center justify-between gap-2 px-4 pt-3.5">{kinds}{session && <ShiftLink id={session.id} />}</div>
          <SaleFind title="Sales" sym={sym} picked={pick?.kind === 'sales' ? pick.id : null} onPick={(id) => setPick({ kind: 'sales', id })} />
        </>
      ) : (
        <RowList kind={kind} head={<div className="flex items-center justify-between gap-2 px-4 pt-3.5">{kinds}{session && <ShiftLink id={session.id} />}</div>}
          sym={sym} pick={pick} onPick={setPick} />
      )}
    </CounterPage>
  )
}

function ShiftLink({ id }: { id: number }) {
  return (
    <a href={`/print/z/${id}`} target="_blank" rel="noreferrer" className="flex items-center gap-1 text-[12.5px] font-semibold text-muted-foreground hover:text-foreground">
      My shift so far <ExternalLink size={13} />
    </a>
  )
}

function RowList({ kind, head, sym, pick, onPick }: {
  kind: 'cash' | 'credit'; head: ReactNode; sym: string; pick: Pick | null; onPick: (p: Pick) => void
}) {
  const [q, setQ] = useState('')
  const dq = useDeferredValue(q.trim())
  const cash = useQuery({ ...moneyReceiptsQuery(dq), enabled: kind === 'cash', placeholderData: keepPreviousData }).data ?? []
  const credit = useQuery({ ...debtReceiptsQuery(dq), enabled: kind === 'credit', placeholderData: keepPreviousData }).data ?? []
  const rows: { key: number; no: string; sub: string; amount: string; p: Pick }[] = kind === 'cash'
    ? cash.map((r) => ({ key: r.id, no: r.receiptNo, sub: `${when(r.createdAt)} · ${r.kindLabel}${r.party ? ` · ${r.party}` : ''}`, amount: r.amount, p: { kind: 'cash', row: r } }))
    : credit.map((r) => ({ key: r.id, no: r.receiptNo, sub: `${when(r.createdAt)} · ${r.customerName} · ${r.method}`, amount: r.amount, p: { kind: 'credit', row: r } }))
  const on = (p: Pick) => pick !== null && pick.kind === p.kind && 'row' in pick && 'row' in p && pick.row.id === p.row.id
  return (
    <>
      {head}
      <PaneTitle title={kind === 'cash' ? 'Cash moves' : 'Credit payments'} sub={`${rows.length} shown`} />
      <div className="relative mx-3 mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input autoFocus aria-label="Find a receipt" placeholder={kind === 'cash' ? 'CR- number, party, locker' : 'Receipt no, customer'} autoComplete="off"
          value={q} onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && rows[0] && onPick(rows[0].p)}
          className={cn(FIELD, 'w-full pl-9')} />
      </div>
      <ul aria-label="Receipts" className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-2.5 pb-2.5">
        {rows.length === 0 && <li className="m-auto p-8 text-center text-faint">Nothing found.</li>}
        {rows.map((r) => (
          <li key={r.key}>
            <button type="button" aria-pressed={on(r.p)} onClick={() => onPick(r.p)}
              className="flex w-full items-center gap-3 rounded-[var(--radius-sm)] px-3 py-2 text-left hover:bg-muted aria-pressed:bg-selected aria-pressed:text-selected-foreground">
              <span className="min-w-0 flex-1">
                <b className="text-[14px] tabular-nums">{r.no}</b>
                <small className="block truncate opacity-70">{r.sub}</small>
              </span>
              <b className="tabular-nums">{money(sym, r.amount)}</b>
            </button>
          </li>
        ))}
      </ul>
    </>
  )
}

function Preview({ pick }: { pick: Pick | null }) {
  const cfg = useSuspenseQuery(configQuery).data
  const sale = useQuery({ ...saleQuery(pick?.kind === 'sales' ? pick.id : 0), enabled: pick?.kind === 'sales' }).data
  if (!pick) return <p className="m-auto max-w-[260px] p-8 text-center text-faint">Pick a receipt to see it and print it again.</p>
  const paper = pick.kind === 'sales'
    ? sale && sale.id === pick.id && <SalePaper s={sale} shop={cfg.shop} sym={cfg.symbol} />
    : pick.kind === 'cash' ? <MoneyPaper r={pick.row} shop={cfg.shop} sym={cfg.symbol} /> : <DebtPaper r={pick.row} shop={cfg.shop} sym={cfg.symbol} />
  const reprint = () => void (pick.kind === 'sales' ? printSale(pick.id) : pick.kind === 'cash' ? printMoneySlip(pick.row.id) : printDebtSlip(pick.row.id))
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="min-h-0 flex-1 overflow-auto bg-muted/60 p-5">{paper}</div>
      <footer className="flex gap-2 border-t border-border p-4">
        {pick.kind === 'sales' && (
          <Button variant="secondary" asChild className="h-11 flex-1">
            <a href={`/print/receipt/${pick.id}`} target="_blank" rel="noreferrer"><ExternalLink size={16} /> Open to print (A4 / PDF)</a>
          </Button>
        )}
        <Button className="h-11 flex-1" onClick={reprint}><Printer size={16} /> Print again</Button>
      </footer>
    </div>
  )
}
