import { useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Printer } from 'lucide-react'
import type { AdminCashflow, CashflowRow } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { countText } from '@/features/counter/queries'
import { rangeQS } from '@/features/counter/range'
import { RangePicker } from '@/features/counter/RangePicker'
import { configQuery } from '@/features/till/api'
import { slipPath } from '@/features/till/print'
import { reprintSlip } from '@/features/till/SlipAsk'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { FLOW_KINDS } from '../lockers/kinds'
import { cashflowLedgerQuery, cashflowQuery } from '../queries'

const route = getRouteApi('/_authed/admin/cashflow')
const when = (iso: string) => new Date(iso).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

/** A signed change: ↓ green in, ↑ red out; a move between your places is neutral. */
export function Delta({ sym, v, moved, className }: { sym: string; v: string; moved?: boolean; className?: string }) {
  const d = num(v)
  return <b className={cn('tabular-nums', moved ? 'text-muted-foreground' : d > 0 ? 'text-success' : 'text-danger', className)}>{d > 0 ? '↓ ' : '↑ '}{money(sym, Math.abs(d))}</b>
}

function Stat({ label, v, sym, tone, hint }: { label: string; v: string; sym: string; tone?: 'in' | 'out'; hint?: string }) {
  return (
    <div className="flex flex-col gap-0.5 px-4 py-3">
      <small className="text-[12px] font-semibold text-muted-foreground">{label}</small>
      <b className={cn('font-display text-xl tabular-nums', tone === 'in' && 'text-success', tone === 'out' && 'text-danger')}>{money(sym, v)}</b>
      {hint && <small className="text-[11px] text-faint">{hint}</small>}
    </div>
  )
}

/** The net position card: each line, then what it comes to. */
export function NetPosition({ d, sym }: { d: AdminCashflow; sym: string }) {
  return (
    <section aria-label="Net position" className={cn(SURFACE, 'flex flex-col p-5')}>
      <h2 className="font-display text-lg font-bold">What the business is worth now</h2>
      <p className="mb-3 text-[13px] text-muted-foreground">An estimate: what you have and are owed, less what you owe.</p>
      <ul className="flex flex-col divide-y divide-border text-sm">
        {d.netLines.map((l) => (
          <li key={l.label} className="flex justify-between gap-3 py-2">
            <span className="text-muted-foreground">{l.sign > 0 ? '+' : '−'} {l.label}</span>
            <span className="tabular-nums">{money(sym, l.amount)}</span>
          </li>
        ))}
      </ul>
      <div className="mt-2 flex items-baseline justify-between border-t-2 border-foreground pt-2">
        <b>Net position</b>
        <b className={cn('font-display text-2xl tabular-nums', num(d.netPosition) < 0 && 'text-danger')}>{money(sym, d.netPosition)}</b>
      </div>
    </section>
  )
}

/** Cash flow (karots-pos /admin/cashflow, 9f17be1): where the money is now, how
 * it moved over the range — moves between your own places counted once, apart
 * from in and out — and every movement, 50 at a time. */
export function CashflowPage() {
  const s = route.useSearch()
  const r = { preset: s.preset ?? 'today', from: s.from ?? '', to: s.to ?? '' }
  const place = s.place ?? ''
  const kind = s.kind ?? ''
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof s>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery(cashflowQuery(r)).data
  const list = useInfiniteQuery(cashflowLedgerQuery(r, place, kind))
  const rows = list.data?.pages.flat() ?? []
  const reprint = (row: CashflowRow) => void reprintSlip(slipPath.money(row.receiptId), row.receiptNo)
  const placeChip = 'flex min-w-0 flex-col items-start rounded-[var(--radius-sm)] border border-hero-foreground/15 bg-hero-foreground/5 px-3 py-2 text-left aria-pressed:border-hero-foreground aria-pressed:bg-hero-foreground/15'

  return (
    <>
      <PageHead title="Cash flow" sub="Where the money is, and how it moved."
        actions={<Button variant="outline" className="h-10 gap-1.5 px-4" asChild><a href={`/print/cashflow?${(rangeQS(r) || '&preset=today').slice(1)}`} target="_blank" rel="noreferrer"><Printer size={16} /> Print</a></Button>} />
      <section className="relative mb-4 overflow-hidden rounded-[var(--radius)] bg-hero bg-[image:var(--hero-art)] p-5 text-hero-foreground md:p-6">
        <small className="text-[13px] font-semibold text-hero-muted">Cash on hand now — lockers and open tills</small>
        <div className="mb-4 font-display text-3xl font-extrabold tabular-nums md:text-4xl">{d ? money(sym, d.cashOnHand) : '…'}</div>
        <div role="group" aria-label="Places" className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5">
          {d?.places.map((p) => (
            <button key={p.value} type="button" aria-pressed={place === p.value} onClick={() => set({ place: place === p.value ? '' : p.value })} className={placeChip}>
              <small className="w-full truncate text-[12px] text-hero-muted">{p.name}</small>
              <b className={cn('tabular-nums', num(p.balance) < 0 && 'text-danger')}>{money(sym, p.balance)}</b>
            </button>
          ))}
        </div>
        {!!d?.plugins.length && d.plugins.map((g) => (
          <p key={g.group} className="mt-3 text-[13px] text-hero-muted">{g.group}: <b className="text-hero-foreground tabular-nums">{money(sym, g.total)}</b> <small>(not cash)</small></p>
        ))}
      </section>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <RangePicker full value={r} onChange={(x) => set({ preset: x.preset || 'today', from: x.from, to: x.to })} />
        {d && <small className="text-[13px] text-muted-foreground">{d.from === d.to ? d.from : `${d.from} – ${d.to}`}</small>}
      </div>
      {d && (
        <div className="mb-4 grid items-start gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
          <section aria-label="In the range" className={cn(SURFACE, 'grid grid-cols-2 divide-border md:grid-cols-4 [&>*]:border-border phone:[&>*:nth-child(-n+2)]:border-b md:divide-x')}>
            <Stat label="↓ Cash sales at the tills" v={d.tillSales} sym={sym} tone="in" hint="net of change" />
            <Stat label="↓ Other money in" v={d.in} sym={sym} tone="in" hint="credit, interest, takings…" />
            <Stat label="↑ Money out" v={d.out} sym={sym} tone="out" hint="expenses, suppliers, refunds" />
            <Stat label="⇄ Moved between your places" v={d.moved} sym={sym} hint="not in, not out" />
            <div className="col-span-2 flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 border-t border-border px-4 py-3 md:col-span-4">
              <span className="font-semibold">Net change{num(d.openings) !== 0 && <small className="ml-2 font-normal text-muted-foreground">incl. {money(sym, d.openings)} opening balances of lockers added</small>}</span>
              <Delta sym={sym} v={d.net} className="font-display text-2xl" />
            </div>
          </section>
          <NetPosition d={d} sym={sym} />
        </div>
      )}
      <div className="mb-3 grid gap-2 sm:flex sm:flex-wrap sm:items-center">
        <select aria-label="Place" value={place} onChange={(e) => set({ place: e.target.value })} className={cn(INPUT, 'appearance-auto sm:w-60')}>
          <option value="">Every place</option>
          {d?.places.map((p) => <option key={p.value} value={p.value}>{p.name}</option>)}
        </select>
        <select aria-label="Kind" value={kind} onChange={(e) => set({ kind: e.target.value })} className={cn(INPUT, 'appearance-auto sm:w-60')}>
          <option value="">Every kind</option>
          {Object.entries(FLOW_KINDS).map(([k, l]) => <option key={k} value={k}>{l}</option>)}
        </select>
        <small className="text-[13px] text-muted-foreground sm:ml-auto">{countText(rows.length, !!list.hasNextPage)} movement{rows.length === 1 ? '' : 's'}</small>
      </div>
      <DataList label="Movements" rows={rows} rowKey={(x) => `${x.at}-${x.place}-${x.kind}-${x.delta}`}
        empty={<EmptyState title={list.isPending ? 'Loading…' : 'No movements in this range'} />}
        footer={list.hasNextPage && (
          <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
            className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60">
            {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
          </button>
        )}
        columns={[
          { key: 'what', header: 'Movement', cell: (x) => (
            <span className="flex flex-col">
              <span className="flex items-center gap-1.5"><b>{FLOW_KINDS[x.kind] ?? x.kind}</b>{x.moved && <Tag tone="muted">moved</Tag>}</span>
              <small className="font-normal text-faint">{when(x.at)}{x.note && ` · ${x.note}`}</small>
            </span>
          ) },
          { key: 'place', header: 'Place', cell: (x) => <span>{x.location}</span> },
          { key: 'receipt', header: 'Receipt', interactive: true, cell: (x) => x.receiptId ? (
            <button type="button" onClick={() => reprint(x)} className="inline-flex items-center gap-1 font-mono text-[12px] text-muted-foreground hover:text-foreground hover:underline"><Printer size={13} /> {x.receiptNo}</button>
          ) : <span className="text-faint">—</span> },
          { key: 'delta', header: 'Change', num: true, cell: (x) => <Delta sym={sym} v={x.delta} moved={x.moved} /> },
        ]} />
    </>
  )
}
