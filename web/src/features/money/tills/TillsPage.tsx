import { useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { INPUT } from '@/components/admin/fields'
import { DataList } from '@/components/admin/DataList'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Tag } from '@/features/admin/products/ProductsPage'
import { countText } from '@/features/counter/queries'
import { RangePicker } from '@/features/counter/RangePicker'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { stamp } from '../format'
import { tillCashiersQuery, tillsQuery } from '../queries'

const route = getRouteApi('/_authed/admin/tills/')

/** Over / short at the close: red short, green over, "exact" at zero. */
export function OverShort({ sym, v, big }: { sym: string; v: string; big?: boolean }) {
  if (v === '') return <span className="text-faint">—</span>
  const n = num(v)
  if (n === 0) return <span className={cn('text-muted-foreground', big && 'font-display text-2xl font-bold')}>exact</span>
  return <b className={cn('tabular-nums', n < 0 ? 'text-danger' : 'text-success', big && 'font-display text-2xl')}>{n < 0 ? 'short ' : 'over '}{money(sym, Math.abs(n))}</b>
}

/** Till sessions (karots-pos /admin/cash-register): every open and close, what
 * the drawer should have held and what was counted. Paged, newest first. */
export function TillsPage() {
  const s = route.useSearch()
  const f = { preset: s.preset ?? '', from: s.from ?? '', to: s.to ?? '', user: s.user ?? 0, open: !!s.open }
  const navigate = route.useNavigate()
  const go = useNavigate()
  const set = (p: Partial<typeof f>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const cashiers = useQuery(tillCashiersQuery).data ?? []
  const list = useInfiniteQuery(tillsQuery(f))
  const rows = list.data?.pages.flat() ?? []
  const chip = 'rounded-full border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

  return (
    <>
      <PageHead title="Till sessions" sub="Every open and close: what the drawer should have held, and what was counted." />
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <RangePicker full value={f} onChange={(r) => set(r)} />
        <select aria-label="Cashier" value={f.user} onChange={(e) => set({ user: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto md:w-56')}>
          <option value={0}>Every cashier</option>
          {cashiers.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
        <button type="button" aria-pressed={f.open} onClick={() => set({ open: !f.open })} className={cn(chip, 'justify-self-start')}>Open now</button>
        <small className="text-[13px] text-muted-foreground md:ml-auto">{countText(rows.length, !!list.hasNextPage)} session{rows.length === 1 ? '' : 's'}</small>
      </div>
      <DataList label="Sessions" rows={rows} rowKey={(r) => r.id} onRow={(r) => void go({ to: '/admin/tills/$id', params: { id: String(r.id) } })}
        empty={<EmptyState title={list.isPending ? 'Loading…' : f.open ? 'No till is open' : 'No sessions in this range'} />}
        footer={list.hasNextPage && (
          <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
            className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60">
            {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
          </button>
        )}
        columns={[
          { key: 'who', header: 'Cashier', cell: (r) => (
            <span className="flex flex-col">
              <span className="flex items-center gap-1.5"><b>{r.cashier}</b>{r.open && <Tag tone="brand">open now</Tag>}</span>
              <small className="font-normal text-faint">{stamp(r.openedAt)}{r.closedAt && ` → ${stamp(r.closedAt)}`}</small>
            </span>
          ) },
          { key: 'opening', header: 'Opening', num: true, wide: true, cell: (r) => money(sym, r.opening) },
          { key: 'expected', header: f.open ? 'In it now' : 'Expected', num: true, cell: (r) => r.expected ? money(sym, r.expected) : '—' },
          { key: 'counted', header: 'Counted', num: true, cell: (r) => r.counted ? money(sym, r.counted) : <span className="text-faint">{r.open ? 'still open' : '—'}</span> },
          { key: 'diff', header: 'Over / short', num: true, cell: (r) => <OverShort sym={sym} v={r.difference} /> },
        ]} />
    </>
  )
}
