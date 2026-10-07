import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { BarCell } from '../kit/BarCell'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { pctText, periodText, share } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { cashiersQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/sales-by-cashier')

/** Sales by cashier (karots-pos /admin/reports/sales-by-cashier): who took
 * what, the discounts they gave, and their share of the takings. */
export function CashiersPage() {
  const r = rangeOf(route.useSearch())
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...cashiersQuery(r), placeholderData: keepPreviousData }).data
  const top = num(d?.rows[0]?.net ?? '0')
  return (
    <ReportPage title="Sales by cashier" sub="What each cashier took, the discounts they gave, and their share."
      range={r} onRange={(x) => void navigate({ search: (p) => ({ ...p, ...x }), replace: true })} caption={d && periodText(d.from, d.to)}
      exportUrl={`/api/v1/admin/reports/sales-by-cashier?${reportQS(r)}`}>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label="Sales" v={d ? String(d.count) : '…'} />
        <Kpi label="Takings" amount={d?.net} v={d ? money(sym, d.net) : '…'} sub="after discounts" />
        <Kpi label="Discounts given" amount={d?.discount} v={d ? money(sym, d.discount) : '…'} />
        <Kpi label="Average sale" v={d ? money(sym, d.avgBasket) : '…'} />
      </KpiStrip>
      <DataList label="Cashiers" rows={d?.rows ?? []} rowKey={(x) => x.cashier + x.count + x.net}
        empty={<EmptyState title={d ? 'No sales in this period' : 'Loading…'} />}
        columns={[
          { key: 'name', header: 'Cashier', cell: (x) => <b>{x.cashier}</b> },
          { key: 'count', header: 'Sales', num: true, cell: (x) => x.count },
          { key: 'net', header: 'Takings', num: true, cell: (x) => <BarCell pct={top > 0 ? (num(x.net) / top) * 100 : null}>{money(sym, x.net)}</BarCell> },
          { key: 'share', header: 'Share', num: true, wide: true, cell: (x) => <span className="text-muted-foreground">{pctText(share(x.net, d?.net ?? '0'))}</span> },
          { key: 'discount', header: 'Discounts', num: true, wide: true, cell: (x) => money(sym, x.discount) },
          { key: 'avg', header: 'Average sale', num: true, wide: true, cell: (x) => money(sym, x.avgBasket) },
        ]} />
    </ReportPage>
  )
}
