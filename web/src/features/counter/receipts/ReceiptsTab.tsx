import { keepPreviousData, useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { ExternalLink, Printer, Search } from 'lucide-react'
import { type ReactNode, useDeferredValue, useState } from 'react'
import type { DebtReceiptRow, MoneyReceiptRow, WarrantyClaimRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { configQuery, drawerQuery } from '@/features/till/api'
import { printDebtSlip, printMoneySlip, printSale, printWarrantySlip } from '@/features/till/print'
import { pickPath } from '@/features/money/receipts/ReceiptSheet'
import { FIELD } from '@/features/till/ui'
import { slots } from '@/plugins/slots'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { countText, debtReceiptsQuery, moneyReceiptsQuery, warrantyClaimsQuery } from '../queries'
import { LATEST, type Range } from '../range'
import { RangePicker } from '../RangePicker'
import { SaleFind } from '../SaleFind'
import { CounterPage, MoreRows, PaneTitle } from '../ui'
import { SlipView } from '@/features/print/SlipView'

type Kind = 'sales' | 'cash' | 'credit' | 'warranty' | `x:${string}`
type Pick = { kind: 'sales'; id: number } | { kind: 'cash'; row: MoneyReceiptRow } | { kind: 'credit'; row: DebtReceiptRow } | { kind: 'warranty'; row: WarrantyClaimRow }

const MOVE_KINDS: [string, string][] = [
  ['', 'All kinds'], ['expense', 'Expenses'], ['supplier_payment', 'Supplier payments'], ['supplier_refund', 'Supplier refunds'],
  ['customer_payment', 'Credit collected'], ['refund', 'Refunds'], ['transfer', 'Drawer moves'],
]
const when = (iso: string) => new Date(iso).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

/** Every slip the till has printed: find it, see it as paper, print it again. */
export function ReceiptsTab() {
  const cfg = useSuspenseQuery(configQuery).data
  const session = useQuery(drawerQuery).data?.summary.session
  const [kind, setKind] = useState<Kind>('sales')
  const [pick, setPick] = useState<Pick | null>(null)
  const sym = cfg.symbol
  const kinds = <Seg label="Receipt kind" value={kind} onChange={(k) => { setKind(k); setPick(null) }} options={[['sales', 'Sales'], ['cash', 'Cash'], ['credit', 'Credit'], ['warranty', 'Warranty'],
    ...slots.tillReceiptKinds.map((k): [Kind, string] => [`x:${k.key}`, k.label])]} />
  const PluginPage = kind.startsWith('x:') ? slots.tillReceiptKinds.find((k) => `x:${k.key}` === kind)?.Page : undefined
  if (PluginPage) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        <div className="px-[14px] pt-[14px]">{kinds}</div>
        <div className="min-h-0 flex-1"><PluginPage /></div>
      </div>
    )
  }
  return (
    <CounterPage flush aside={<Preview pick={pick} />}>
      {kind === 'sales' ? (
        <>
          <div className="flex items-center justify-between gap-2 px-4 pt-3.5">{kinds}{session && <ShiftLink id={session.id} />}</div>
          <SaleFind title="Sales" sym={sym} picked={pick?.kind === 'sales' ? pick.id : null} onPick={(id) => setPick({ kind: 'sales', id })} />
        </>
      ) : (
        <RowList kind={kind as 'cash' | 'credit' | 'warranty'} head={<div className="flex items-center justify-between gap-2 px-4 pt-3.5">{kinds}{session && <ShiftLink id={session.id} />}</div>}
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
  kind: 'cash' | 'credit' | 'warranty'; head: ReactNode; sym: string; pick: Pick | null; onPick: (p: Pick) => void
}) {
  const [q, setQ] = useState('')
  const [range, setRange] = useState<Range>(LATEST)
  const [moveKind, setMoveKind] = useState('')
  const dq = useDeferredValue(q.trim())
  const cashQ = useInfiniteQuery({ ...moneyReceiptsQuery(dq, moveKind, range), enabled: kind === 'cash', placeholderData: keepPreviousData })
  const creditQ = useInfiniteQuery({ ...debtReceiptsQuery(dq, range), enabled: kind === 'credit', placeholderData: keepPreviousData })
  const claimsQ = useInfiniteQuery({ ...warrantyClaimsQuery(dq, range), enabled: kind === 'warranty', placeholderData: keepPreviousData })
  const list = kind === 'cash' ? cashQ : kind === 'credit' ? creditQ : claimsQ
  const cash = cashQ.data?.pages.flat() ?? []
  const credit = creditQ.data?.pages.flat() ?? []
  const claims = claimsQ.data?.pages.flat() ?? []
  const rows: { key: number; no: string; sub: string; amount: string | null; p: Pick }[] = kind === 'cash'
    ? cash.map((r) => ({ key: r.id, no: r.receiptNo, sub: `${when(r.createdAt)} · ${r.kindLabel}${r.party ? ` · ${r.party}` : ''}`, amount: r.amount, p: { kind: 'cash', row: r } }))
    : kind === 'credit'
      ? credit.map((r) => ({ key: r.id, no: r.receiptNo, sub: `${when(r.createdAt)} · ${r.customerName} · ${r.method}`, amount: r.amount, p: { kind: 'credit', row: r } }))
      : claims.map((r) => ({ key: r.id, no: r.productName, sub: `${when(r.at)} · ${r.oldSerial || 'by receipt'}${r.customerName ? ` · ${r.customerName}` : ''}`, amount: null, p: { kind: 'warranty', row: r } }))
  const on = (p: Pick) => pick !== null && pick.kind === p.kind && 'row' in pick && 'row' in p && pick.row.id === p.row.id
  return (
    <>
      {head}
      <PaneTitle title={kind === 'cash' ? 'Cash moves' : kind === 'credit' ? 'Credit payments' : 'Warranty claims'} sub={`${countText(rows.length, list.hasNextPage)} shown`}
        right={(
          <span className="flex items-center gap-1.5">
            {kind === 'cash' && (
              <select aria-label="Kind of move" value={moveKind} onChange={(e) => setMoveKind(e.target.value)} className={cn(FIELD, 'h-8 appearance-auto px-2 text-[13px]')}>
                {MOVE_KINDS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
              </select>
            )}
            <RangePicker value={range} onChange={setRange} />
          </span>
        )} />
      <div className="relative mx-3 mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input autoFocus aria-label="Find a receipt" placeholder={kind === 'cash' ? 'CR- number, party, locker' : kind === 'credit' ? 'Receipt no, customer' : 'Serial or customer'} autoComplete="off"
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
              {r.amount && <b className="tabular-nums">{money(sym, r.amount)}</b>}
            </button>
          </li>
        ))}
        <MoreRows list={list} />
      </ul>
    </>
  )
}

function Preview({ pick }: { pick: Pick | null }) {
  const cfg = useSuspenseQuery(configQuery).data
  const [size, setSize] = useState<'80' | '58'>(cfg.receiptWidth === '58' ? '58' : '80')
  if (!pick) return <p className="m-auto max-w-[260px] p-8 text-center text-faint">Pick a receipt to see it and print it again.</p>
  const path = pickPath(pick)
  const paper = path ? <SlipView path={path} size={size} /> : <p className="text-center text-faint">A receipt claim prints no slip.</p>
  const reprint = () => void (pick.kind === 'sales' ? printSale(pick.id, false, size) : pick.kind === 'cash' ? printMoneySlip(pick.row.id, false, size)
    : pick.kind === 'credit' ? printDebtSlip(pick.row.id, false, size) : printWarrantySlip(pick.row.id, size))
  const canReprint = pick.kind !== 'warranty' || pick.row.replacementSerial !== ''
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="min-h-0 flex-1 overflow-auto bg-muted/60 p-5">
        <div className="mb-3 flex justify-end"><Seg label="Paper width" value={size} onChange={setSize} options={[['80', '80 mm'], ['58', '58 mm']]} /></div>
        {paper}
      </div>
      <footer className="flex gap-2 border-t border-border p-4">
        {pick.kind === 'sales' && (
          <Button variant="secondary" asChild className="h-11 flex-1">
            <a href={`/print/receipt/${pick.id}`} target="_blank" rel="noreferrer"><ExternalLink size={16} /> Print A4 / PDF</a>
          </Button>
        )}
        {canReprint && <Button className="h-11 flex-1" onClick={reprint}><Printer size={16} /> Print again · {size} mm</Button>}
      </footer>
    </div>
  )
}
