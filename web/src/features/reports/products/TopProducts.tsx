import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Sparkles } from 'lucide-react'
import { DataList } from '@/components/admin/DataList'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { Seg } from '@/features/till/bits'
import { configQuery } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { BarCell } from '../kit/BarCell'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { pctText, periodText, share } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { topProductsQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/top-products')
type Order = 'revenue' | 'qty' | 'profit'
const WORD: Record<Order, string> = { revenue: 'revenue', qty: 'units sold', profit: 'profit' }

/** Top products (karots-pos /admin/reports/top-products): the best sellers by
 * revenue, units or profit, each as a share of everything sold, and how few
 * products make 80% of it. Tap one for its sales over time. */
export function TopProductsPage() {
  const s = route.useSearch()
  const r = rangeOf(s)
  const order = (s.order ?? 'revenue') as Order
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof s>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...topProductsQuery({ ...r, order }), placeholderData: keepPreviousData }).data
  const metric = (x: { qty: string; revenue: string; profit: string }) => (order === 'qty' ? x.qty : x[order])
  const whole = d ? metric(d) : '0'
  const top = d?.rows[0] ? num(metric(d.rows[0])) : 0
  const runs = (d?.rows ?? []).reduce<number[]>((acc, x) => [...acc, (acc.at(-1) ?? 0) + num(metric(x))], [])
  const rows = (d?.rows ?? []).map((x, i) => ({ ...x, rank: i + 1, bar: top > 0 ? (num(metric(x)) / top) * 100 : null, cum: share(String(runs[i]), whole) }))
  const pareto = rows.find((x) => (x.cum ?? 0) >= 80)

  return (
    <ReportPage title="Top products" sub="Your best sellers, and how much of the takings they carry."
      range={r} onRange={(x) => set(x)} caption={d && periodText(d.from, d.to)} exportUrl={`/api/v1/admin/reports/top-products?${reportQS({ ...r, order })}`}
      filters={<Seg label="Rank by" value={order} onChange={(o) => set({ order: o })} options={[['revenue', 'Revenue'], ['qty', 'Units'], ['profit', 'Profit']]} />}>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label="Products sold" v={d ? String(d.products) : '…'} />
        <Kpi label="Units" v={d ? qtyText(d.qty) : '…'} />
        <Kpi label="Revenue" amount={d?.revenue} v={d ? money(sym, d.revenue) : '…'} />
        <Kpi label="Profit" amount={d?.profit} tone="sign" v={d ? money(sym, d.profit) : '…'} sub={d && `${d.margin}% margin`} />
      </KpiStrip>
      {d && rows.length > 0 && (
        <p className={cn(SURFACE, 'flex items-center gap-2.5 px-4 py-3 text-sm')}>
          <Sparkles size={18} className="flex-none text-brand" />
          {pareto
            ? <span><b>{pareto.rank} of {d.products}</b> product{d.products === 1 ? '' : 's'} make 80% of your {WORD[order]}.</span>
            : <span>The top {rows.length} make {pctText(rows[rows.length - 1].cum)} of your {WORD[order]}.</span>}
        </p>
      )}
      <DataList label="Products" rows={rows} rowKey={(x) => x.productId}
        onRow={(x) => void navigate({ to: '/admin/reports/product-sales', search: { preset: s.preset, from: s.from, to: s.to, product: x.productId } })}
        empty={<EmptyState title={d ? 'Nothing sold in this period' : 'Loading…'} />}
        columns={[
          { key: 'name', header: 'Product', cell: (x) => <span className="flex items-baseline gap-2"><small className="w-6 flex-none text-right tabular-nums text-faint">{x.rank}</small><b>{x.name}</b></span> },
          { key: 'qty', header: 'Units', num: true, wide: order !== 'qty', cell: (x) => (order === 'qty' ? <BarCell pct={x.bar}>{qtyText(x.qty)}</BarCell> : qtyText(x.qty)) },
          { key: 'revenue', header: 'Revenue', num: true, cell: (x) => (order === 'revenue' ? <BarCell pct={x.bar}>{money(sym, x.revenue)}</BarCell> : money(sym, x.revenue)) },
          { key: 'profit', header: 'Profit', num: true, wide: order !== 'profit', cell: (x) => (order === 'profit'
            ? <BarCell pct={x.bar} tone="success">{money(sym, x.profit)}</BarCell> : <span className={cn(num(x.profit) < 0 && 'text-danger')}>{money(sym, x.profit)}</span>) },
          { key: 'margin', header: 'Margin', num: true, wide: true, cell: (x) => <span className={cn(num(x.margin) < 0 && 'text-danger')}>{x.margin}%</span> },
          { key: 'cum', header: 'Share · running', num: true, wide: true, cell: (x) => <span className="text-muted-foreground">{pctText(share(metric(x), whole))} · {pctText(x.cum)}</span> },
        ]} />
    </ReportPage>
  )
}
