import { keepPreviousData, useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { ChartCard, ChartTable } from '@/components/charts/ChartCard'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState } from '@/components/admin/PageHead'
import { Tag } from '@/features/admin/products/ProductsPage'
import { countText } from '@/features/counter/queries'
import { trendOption } from '@/features/money/finance/charts'
import { stamp } from '@/features/money/format'
import { adminSalesQuery, salesQS } from '@/features/money/queries'
import { METHODS, STATUS, STATUS_TAG } from '@/features/sales/filters'
import { SaleSheet } from '@/features/sales/SaleSheet'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { delta, periodText } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf } from '../kit/search'
import { salesReportQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/sales')
const HOURS = Array.from({ length: 24 }, (_, h) => String(h))
const hh = (h: string) => `${h.padStart(2, '0')}:00`

/** Sales report (karots-pos /admin/reports/sales): totals over the whole range
 * with ▲▼ against the period before, an hour-of-day window, the trend, and
 * every receipt with its cost and profit — tap one for its lines. */
export function SalesReportPage() {
  const s = route.useSearch()
  const f = { ...rangeOf(s), status: s.status ?? '', method: s.method ?? '', q: '', fromHour: s.fromHour ?? '', toHour: s.toHour ?? '' }
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof s>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...salesReportQuery(f), placeholderData: keepPreviousData }).data
  const list = useInfiniteQuery({ ...adminSalesQuery(f), placeholderData: keepPreviousData })
  const rows = list.data?.pages.flat() ?? []
  const [open, setOpen] = useState<number | null>(null)
  const trend = useMemo(() => d && trendOption(d.trend, sym, d.grain), [d, sym])
  const sel = cn(INPUT, 'appearance-auto md:w-40')
  const hourSel = (label: string, v: string, key: 'fromHour' | 'toHour') => (
    <select aria-label={label} value={v} onChange={(e) => set({ [key]: e.target.value })} className={cn(INPUT, 'appearance-auto md:w-[150px]')}>
      <option value="">{key === 'fromHour' ? 'From any hour' : 'To any hour'}</option>
      {HOURS.map((h) => <option key={h} value={h}>{hh(h)}</option>)}
    </select>
  )
  const sum = d?.sum
  const grainWord = { day: 'day', week: 'week', month: 'month' }[d?.grain ?? 'day']

  return (
    <ReportPage rows={list} title="Sales" sub="Every sale in a period — what came in, what it cost, and how it compares with the period before."
      range={f} onRange={(r) => set(r)} caption={d && periodText(d.from, d.to)} exportUrl={`/api/v1/admin/sales/export?${salesQS(f)}`}
      filters={(
        <div className="grid grid-cols-2 gap-2 md:flex">
          <select aria-label="Status" value={f.status} onChange={(e) => set({ status: e.target.value })} className={sel}>{STATUS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</select>
          <select aria-label="Payment" value={f.method} onChange={(e) => set({ method: e.target.value })} className={sel}>{METHODS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</select>
          {hourSel('From hour', f.fromHour, 'fromHour')}
          {hourSel('To hour', f.toHour, 'toHour')}
        </div>
      )}>
      <KpiStrip label="Headline" cols={5}>
        <Kpi label="Sales" v={sum ? String(sum.count) : '…'} change={sum && d ? delta(String(sum.count), String(d.prev.count)) : null} />
        <Kpi label="Revenue" amount={sum?.revenue} v={sum ? money(sym, sum.revenue) : '…'} change={sum && d ? delta(sum.revenue, d.prev.revenue) : null} sub="after returns" />
        <Kpi label="Profit" amount={sum?.profit} tone="sign" v={sum ? money(sym, sum.profit) : '…'} change={sum && d ? delta(sum.profit, d.prev.profit) : null} />
        <Kpi label="Margin" v={sum ? `${sum.margin}%` : '…'} sub={sum && num(sum.discount) > 0 ? `${money(sym, sum.discount)} given off` : undefined} />
        <Kpi label="Average sale" amount={sum?.avgBasket} v={sum ? money(sym, sum.avgBasket) : '…'} change={sum && d ? delta(sum.avgBasket, d.prev.avgBasket) : null} className="phone:col-span-2" />
      </KpiStrip>
      {d && trend && (
        <ChartCard title="Revenue and profit" sub={`By ${grainWord}`} option={trend} label={`Revenue and profit by ${grainWord}`} empty={!d.trend.length}
          table={<ChartTable head={['Period', 'Revenue', 'Profit']} rows={d.trend.map((p) => [p.day, money(sym, p.revenue), money(sym, p.profit)])} />} />
      )}
      <div>
        <div className="mb-2 text-right text-[13px] text-muted-foreground">{countText(rows.length, !!list.hasNextPage)} shown · tap one for its lines</div>
        <DataList label="Receipts" rows={rows} rowKey={(r) => r.id} onRow={(r) => setOpen(r.id)}
          empty={<EmptyState title={list.isPending ? 'Loading…' : 'No sales in this period'} />}
          footer={list.hasNextPage && (
            <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
              className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60 print:hidden">
              {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
            </button>
          )}
          columns={[
            { key: 'sale', header: 'Sale', cell: (r) => (
              <span className="flex flex-col">
                <span className="flex items-center gap-1.5"><b className="tabular-nums">{r.receiptNo}</b>{STATUS_TAG[r.status] && <Tag tone={STATUS_TAG[r.status][1]}>{STATUS_TAG[r.status][0]}</Tag>}</span>
                <small className="font-normal text-faint">{stamp(r.createdAt)} · {r.cashierName}</small>
              </span>
            ) },
            { key: 'total', header: 'Total', num: true, cell: (r) => <b>{money(sym, r.total)}</b> },
            { key: 'cogs', header: 'Cost', num: true, wide: true, cell: (r) => <span className="text-muted-foreground">{money(sym, r.cogs)}</span> },
            { key: 'profit', header: 'Profit', num: true, cell: (r) => <span className={cn(num(r.profit) < 0 && 'text-danger')}>{money(sym, r.profit)}</span> },
            { key: 'margin', header: 'Margin', num: true, wide: true, cell: (r) => <span className={cn(num(r.margin) < 0 && 'text-danger')}>{r.margin}%</span> },
          ]} />
      </div>
      {open && <SaleSheet id={open} onClose={() => setOpen(null)} />}
    </ReportPage>
  )
}
