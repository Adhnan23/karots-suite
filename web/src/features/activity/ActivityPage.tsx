import { infiniteQueryOptions, keepPreviousData, queryOptions, useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '@/api/client'
import type { ActivityFilters, ActivityPage as Page } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState } from '@/components/admin/PageHead'
import { Tag } from '@/features/admin/products/ProductsPage'
import { stamp } from '@/features/money/format'
import { periodText } from '@/features/reports/kit/numbers'
import { ReportPage } from '@/features/reports/kit/ReportPage'
import { rangeOf, reportQS } from '@/features/reports/kit/search'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'

const route = getRouteApi('/_authed/admin/activity')
type F = { preset: string; from: string; to: string; user: string; source: string; q: string }
const filtersQuery = queryOptions({ queryKey: ['admin', 'activity-filters'], queryFn: () => api<ActivityFilters>('/admin/activity/filters') })
const activityQuery = (f: F) => infiniteQueryOptions({
  queryKey: ['admin', 'activity', f], initialPageParam: '',
  queryFn: ({ pageParam }) => api<Page>(`/admin/activity?${reportQS(f)}${pageParam ? `&before=${encodeURIComponent(pageParam)}` : ''}`),
  getNextPageParam: (last: Page) => last.next || undefined,
})

/** Activity (karots-pos /admin/activity, with the Audit log as its Changes
 * source): who did what — changes, stock moves, the cash drawer and lockers —
 * by person, kind, period and words. Admins only. */
export function ActivityPage() {
  const s = route.useSearch()
  const f: F = { ...rangeOf(s), user: s.user ?? '', source: s.source ?? '', q: s.q ?? '' }
  const navigate = route.useNavigate()
  const set = (p: Partial<F>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const opts = useQuery(filtersQuery).data
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q.trim(), 250)
  useEffect(() => { if (dq !== f.q) set({ q: dq }) }, [dq]) // eslint-disable-line react-hooks/exhaustive-deps
  const list = useInfiniteQuery({ ...activityQuery(f), placeholderData: keepPreviousData })
  const rows = list.data?.pages.flatMap((p) => p.rows) ?? []
  const label = (src: string) => opts?.sources.find((o) => o.id === src)?.label ?? src
  const sel = cn(INPUT, 'appearance-auto md:w-48')
  return (
    <ReportPage title="Activity" sub="Who did what — changes, stock moves, the cash drawer and lockers."
      range={f} onRange={(r) => set(r)} caption={rows.length ? undefined : periodText(f.from || '', f.to || '') || undefined}
      exportUrl={`/api/v1/admin/activity?${reportQS(f)}`}
      filters={(
        <>
          <div className="relative md:w-64">
            <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
            <input aria-label="Search activity" placeholder="Search what was done" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
          </div>
          <div className="grid grid-cols-2 gap-2 md:flex">
            <select aria-label="Who" value={f.user} onChange={(e) => set({ user: e.target.value })} className={sel}>
              <option value="">Everyone</option>
              {opts?.users.map((u) => <option key={u.id} value={u.id}>{u.name}</option>)}
            </select>
            <select aria-label="Kind" value={f.source} onChange={(e) => set({ source: e.target.value })} className={sel}>
              <option value="">Everything</option>
              {opts?.sources.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}
            </select>
          </div>
        </>
      )}>
      <DataList label="Activity" rows={rows} rowKey={(r) => `${r.when}-${r.source}-${r.action}-${r.detail}`}
        empty={<EmptyState title={list.isPending ? 'Loading…' : 'Nothing matches'} />}
        footer={list.hasNextPage && (
          <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
            className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60 print:hidden">
            {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
          </button>
        )}
        columns={[
          { key: 'what', header: 'What', cell: (r) => (
            <span className="flex flex-col">
              <span className="flex flex-wrap items-center gap-1.5"><b>{r.action.replaceAll('_', ' ')}</b><Tag tone="muted">{label(r.source)}</Tag></span>
              <small className="font-normal text-muted-foreground">{r.detail}</small>
            </span>
          ) },
          { key: 'who', header: 'Who', cell: (r) => <span className="flex flex-col"><span>{r.user || '—'}</span><small className="text-faint">{stamp(r.when)}</small></span> },
          { key: 'amt', header: 'Money', num: true, cell: (r) => (r.amount
            ? <b className={cn('tabular-nums', num(r.amount) > 0 ? 'text-success' : 'text-danger')}>{num(r.amount) > 0 ? '↓ ' : '↑ '}{money(sym, String(Math.abs(num(r.amount))))}</b>
            : <span className="text-faint">—</span>) },
        ]} />
    </ReportPage>
  )
}
