import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { configQuery } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { BarCell } from '../kit/BarCell'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { periodText } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { serviceProfitQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/service-profit')

/** Service profit (karots-pos /admin/reports/service-profit): did each service
 * pay for itself — takings, what its recipe used, its own expenses. */
export function ServiceProfitPage() {
  const r = rangeOf(route.useSearch())
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...serviceProfitQuery(r), placeholderData: keepPreviousData }).data
  const top = Math.max(0, ...(d?.rows.map((x) => num(x.revenue)) ?? []))
  return (
    <ReportPage title="Service profit" sub="Each service’s takings, less what its recipe used and the expenses booked against it."
      range={r} onRange={(x) => void navigate({ search: (p) => ({ ...p, ...x }), replace: true })} caption={d && periodText(d.from, d.to)}
      exportUrl={`/api/v1/admin/reports/service-profit?${reportQS(r)}`}>
      <KpiStrip label="Headline" cols={3}>
        <Kpi label="Takings" v={d ? money(sym, d.revenue) : '…'} />
        <Kpi label="↑ Expenses booked to services" amount={d?.expenses} tone="out" v={d ? money(sym, d.expenses) : '…'} />
        <Kpi label="Net" amount={d?.net} tone="sign" v={d ? money(sym, d.net) : '…'} className="max-md:col-span-2" />
      </KpiStrip>
      <DataList label="Services" rows={d?.rows ?? []} rowKey={(x) => x.productId}
        empty={<EmptyState title={d ? 'No services sold in this period' : 'Loading…'} />}
        columns={[
          { key: 'name', header: 'Service', cell: (x) => <span className="flex flex-col"><b>{x.name}</b><small className="font-normal text-faint">{qtyText(x.units)} sold</small></span> },
          { key: 'rev', header: 'Takings', num: true, cell: (x) => <BarCell pct={top > 0 ? (num(x.revenue) / top) * 100 : null}>{money(sym, x.revenue)}</BarCell> },
          { key: 'cogs', header: 'Ingredients', num: true, wide: true, cell: (x) => money(sym, x.cogs) },
          { key: 'exp', header: 'Expenses', num: true, wide: true, cell: (x) => money(sym, x.expenses) },
          { key: 'net', header: 'Net', num: true, cell: (x) => <b className={cn(num(x.net) < 0 && 'text-danger')}>{money(sym, x.net)}</b> },
          { key: 'margin', header: 'Margin', num: true, wide: true, cell: (x) => <span className={cn(num(x.margin) < 0 && 'text-danger')}>{x.margin}%</span> },
        ]} />
    </ReportPage>
  )
}
