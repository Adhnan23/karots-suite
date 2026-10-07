import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { BarCell } from '../kit/BarCell'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { periodText } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { categoryProfitQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/profit-by-category')

/** Profit by category (karots-pos /admin/reports/profit-by-category): revenue,
 * cost and profit per category, each parent holding its sub-categories. */
export function CategoryProfitPage() {
  const s = route.useSearch()
  const r = rangeOf(s)
  const q = { ...r, category: s.category ? String(s.category) : '' }
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof s>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...categoryProfitQuery(q), placeholderData: keepPreviousData }).data
  const top = Math.max(0, ...(d?.nodes.filter((n) => n.depth === 0).map((n) => num(n.profit)) ?? []))
  return (
    <ReportPage title="Profit by category" sub="What each category sold, what it cost, and what it made — sub-categories inside their parent."
      range={r} onRange={(x) => set(x)} caption={d && periodText(d.from, d.to)} exportUrl={`/api/v1/admin/reports/profit-by-category?${reportQS(q)}`}
      filters={<div className="md:w-72"><CategoryPicker label="Category" noneLabel="Every category" value={Number(q.category) || 0} onChange={(id) => set({ category: id })} /></div>}>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label="Revenue" amount={d?.revenue} v={d ? money(sym, d.revenue) : '…'} />
        <Kpi label="Cost of goods" v={d ? money(sym, d.cogs) : '…'} />
        <Kpi label="Profit" amount={d?.profit} tone="sign" v={d ? money(sym, d.profit) : '…'} />
        <Kpi label="Margin" v={d ? `${d.margin}%` : '…'} />
      </KpiStrip>
      <DataList label="Categories" rows={d?.nodes ?? []} rowKey={(x) => x.id}
        onRow={(x) => set({ category: x.id })}
        empty={<EmptyState title={d ? 'Nothing sold in this period' : 'Loading…'} />}
        columns={[
          { key: 'name', header: 'Category', cell: (x) => (
            <span className={cn('block', x.depth === 0 ? 'font-bold' : 'text-muted-foreground')} style={{ paddingLeft: `${x.depth * 18}px` }}>{x.depth > 0 && '└ '}{x.name}</span>
          ) },
          { key: 'rev', header: 'Revenue', num: true, cell: (x) => money(sym, x.revenue) },
          { key: 'cogs', header: 'Cost', num: true, wide: true, cell: (x) => <span className="text-muted-foreground">{money(sym, x.cogs)}</span> },
          { key: 'profit', header: 'Profit', num: true, cell: (x) => (x.depth === 0
            ? <BarCell pct={top > 0 ? (num(x.profit) / top) * 100 : null} tone="success">{money(sym, x.profit)}</BarCell>
            : <span className={cn(num(x.profit) < 0 && 'text-danger')}>{money(sym, x.profit)}</span>) },
          { key: 'margin', header: 'Margin', num: true, wide: true, cell: (x) => <span className={cn(num(x.margin) < 0 && 'text-danger')}>{x.margin}%</span> },
        ]} />
    </ReportPage>
  )
}
