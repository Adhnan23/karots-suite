import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { configQuery } from '@/features/till/api'
import { money } from '@/lib/money'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { periodText } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { taxQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/tax')

/** Tax summary (karots-pos /admin/reports/tax, tax-registered shops): tax
 * charged at sale per day, with the taxable amount. */
export function TaxPage() {
  const r = rangeOf(route.useSearch())
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...taxQuery(r), placeholderData: keepPreviousData }).data
  return (
    <ReportPage title="Tax" sub="Tax charged at the till, day by day, with the amount it was charged on."
      range={r} onRange={(x) => void navigate({ search: (p) => ({ ...p, ...x }), replace: true })} caption={d && periodText(d.from, d.to)}
      exportUrl={`/api/v1/admin/reports/tax?${reportQS(r)}`}>
      <KpiStrip label="Headline" cols={3}>
        <Kpi label="Tax charged" v={d ? money(sym, d.tax) : '…'} />
        <Kpi label="Taxable sales" v={d ? money(sym, d.base) : '…'} />
        <Kpi label="Sales" v={d ? String(d.count) : '…'} className="max-md:col-span-2" />
      </KpiStrip>
      <DataList label="Days" rows={d?.rows ?? []} rowKey={(x) => x.day}
        empty={<EmptyState title={d ? 'No sales in this period' : 'Loading…'} />}
        columns={[
          { key: 'day', header: 'Day', cell: (x) => <b>{x.day}</b> },
          { key: 'n', header: 'Sales', num: true, cell: (x) => x.count },
          { key: 'base', header: 'Taxable', num: true, cell: (x) => money(sym, x.base) },
          { key: 'tax', header: 'Tax', num: true, cell: (x) => <b>{money(sym, x.tax)}</b> },
        ]} />
    </ReportPage>
  )
}
