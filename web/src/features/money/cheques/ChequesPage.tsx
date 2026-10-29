import { useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { ChequeRow } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { MethodBadge } from '@/components/admin/MethodBadge'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { countText } from '@/features/counter/queries'
import { RangePicker } from '@/features/counter/RangePicker'
import { Seg } from '@/features/till/bits'
import { configQuery } from '@/features/till/api'
import { money } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { ChequeSheet } from './ChequeSheet'
import { chequeSummaryQuery, chequesQuery } from './queries'

const route = getRouteApi('/_authed/admin/cheques/')
const day = (d: string) => new Date(`${d}T00:00`).toLocaleDateString([], { day: 'numeric', month: 'short', year: '2-digit' })
const STATUSES: [string, string][] = [['pending', 'Pending'], ['cleared', 'Cleared'], ['bounced', 'Bounced'], ['cancelled', 'Cancelled'], ['', 'All']]

/** When a cheque is due: "in 3 days", "today", "5 days ago". */
export function dueText(days: number) {
  if (days === 0) return 'today'
  if (days === 1) return 'tomorrow'
  if (days === -1) return 'yesterday'
  return days > 0 ? `in ${days} days` : `${-days} days ago`
}

/** Money → Cheques: cheques taken from customers and written to suppliers /
 * for expenses. Every one waits as Pending until someone marks it Cleared,
 * Bounced or Cancelled — nothing changes by itself. */
export function ChequesPage() {
  const s = route.useSearch()
  const f = { dir: s.dir === 'out' ? ('out' as const) : ('in' as const), status: s.status ?? 'pending', preset: s.preset ?? '', from: s.from ?? '', to: s.to ?? '', q: s.q ?? '', due: s.due === '1' }
  const navigate = route.useNavigate()
  const set = (p: Partial<Record<keyof typeof f, string>>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q, 250)
  useEffect(() => { if (dq !== f.q) set({ q: dq }) }, [dq]) // eslint-disable-line react-hooks/exhaustive-deps
  const sum = useQuery(chequeSummaryQuery).data
  const list = useInfiniteQuery(chequesQuery(f))
  const rows = list.data?.pages.flat() ?? []
  const [open, setOpen] = useState<number | null>(null)
  const chip = 'whitespace-nowrap rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'
  const received = f.dir === 'in'

  return (
    <>
      <PageHead title="Cheques" sub="Taken and written — each waits until you mark it Cleared, Bounced or Cancelled." />
      <section className={cn(SURFACE, 'mb-3 grid grid-cols-1 gap-3 p-4 sm:grid-cols-3')} aria-label="Cheque totals">
        <Total label="Cheques in hand" value={money(sym, sum?.inHand ?? '0')} hint={sum && sum.toDeposit.count > 0 ? `${sum.toDeposit.count} to deposit now` : 'received, not cleared yet'} />
        <Total label="To be cashed" value={money(sym, sum?.given ?? '0')} hint="your cheques the bank hasn't paid yet" />
        <Total label="Due this week" value={money(sym, sum?.dueWeek ?? '0')} hint={sum?.short.length ? `${sum.short.map((b) => b.name).join(', ')} will go below zero` : 'your cheques dated within 7 days'} warn={!!sum?.short.length} />
      </section>
      <div className="mb-3 flex flex-col gap-2 md:flex-row md:flex-wrap md:items-center">
        <Seg label="Cheques" value={f.dir} options={[['in', 'Received'], ['out', 'Given']]} onChange={(dir) => set({ dir, due: '' })} />
        <RangePicker full value={f} onChange={(r) => set(r)} />
        <div className="relative md:w-72">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search cheques" placeholder={received ? 'Customer, bank or number' : 'Supplier, expense or number'} value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
      </div>
      <div role="group" aria-label="Status" className="-mx-1 mb-3 flex gap-1.5 overflow-x-auto px-1 pb-1">
        {STATUSES.map(([v, l]) => <button key={v} type="button" aria-pressed={f.status === v && !f.due} onClick={() => set({ status: v, due: '' })} className={chip}>{l}</button>)}
        {f.due && <button type="button" aria-pressed className={chip} onClick={() => set({ due: '' })}>{received ? 'To deposit' : 'Due in 3 days'} ×</button>}
      </div>
      <div className="mb-2 text-right text-[13px] text-muted-foreground">{countText(rows.length, !!list.hasNextPage)} shown</div>
      <DataList label="Cheques" rows={rows} rowKey={(c) => c.id} onRow={(c) => setOpen(c.id)}
        empty={<EmptyState title={list.isPending ? 'Loading…' : 'No cheques here'}>{!list.isPending && (received ? 'Cheques customers give you show here until they clear.' : 'Cheques you write to suppliers and for expenses show here until the bank pays them.')}</EmptyState>}
        footer={list.hasNextPage && (
          <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
            className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60">
            {list.isFetchingNextPage ? 'Loading…' : 'Show more'}
          </button>
        )}
        columns={[
          { key: 'date', header: 'Cheque date', cell: (c) => <DateCell c={c} /> },
          { key: 'who', header: received ? 'From' : 'To', cell: (c) => (
            <span className="flex min-w-0 flex-col"><b className="truncate">{c.who || '—'}</b>
              <small className="truncate font-normal text-faint">{received ? c.bank : c.lockerName} · #{c.number}</small></span>
          ) },
          { key: 'status', header: 'Status', cell: (c) => <MethodBadge method="cheque" label={c.status === 'pending' ? 'Pending' : cap(c.status)} status={c.status === 'pending' ? undefined : c.status} /> },
          { key: 'into', header: received ? 'Cleared into' : 'Drawn on', wide: true, cell: (c) => <span className="text-muted-foreground">{c.lockerName || '—'}</span> },
          { key: 'amount', header: 'Amount', num: true, cell: (c) => <b className="tabular-nums text-cheque">{received ? '↓' : '↑'} {money(sym, c.amount)}</b> },
        ]} />
      {open !== null && <ChequeSheet id={open} sym={sym} onClose={() => setOpen(null)} />}
    </>
  )
}

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)

function DateCell({ c }: { c: ChequeRow }) {
  const late = c.status === 'pending' && c.days <= 0
  return (
    <span className="flex flex-col">
      <b className="tabular-nums">{day(c.chequeDate)}</b>
      {c.status === 'pending' && <small className={cn('font-normal', late ? 'font-semibold text-warning' : 'text-faint')}>{dueText(c.days)}</small>}
    </span>
  )
}

function Total({ label, value, hint, warn }: { label: string; value: string; hint: string; warn?: boolean }) {
  return (
    <div className="min-w-0">
      <small className="text-muted-foreground">{label}</small>
      <b className="block font-display text-2xl tabular-nums text-cheque">{value}</b>
      <small className={cn('block truncate', warn ? 'font-semibold text-warning' : 'text-faint')}>{hint}</small>
    </div>
  )
}
