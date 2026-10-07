import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { Tag } from '@/features/admin/products/ProductsPage'
import { stamp } from '@/features/money/format'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { periodText } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { purchasesReportQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/purchases')

/** Purchases (karots-pos /admin/reports/purchases): the deliveries received in
 * a period, what they came to, and what's still to pay. Tap one for its GRN. */
export function PurchasesReportPage() {
  const r = rangeOf(route.useSearch())
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...purchasesReportQuery(r), placeholderData: keepPreviousData }).data
  return (
    <ReportPage title="Purchases" sub="Deliveries received in a period, and what’s still to pay on them."
      range={r} onRange={(x) => void navigate({ search: (p) => ({ ...p, ...x }), replace: true })} caption={d && periodText(d.from, d.to)}
      exportUrl={`/api/v1/admin/reports/purchases?${reportQS(r)}`}>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label="Deliveries" v={d ? String(d.count) : '…'} />
        <Kpi label="Came to" amount={d?.total} v={d ? money(sym, d.total) : '…'} sub={d && num(d.credited) > 0 ? `${money(sym, d.credited)} sent back` : undefined} />
        <Kpi label="↑ Paid" v={d ? money(sym, d.paid) : '…'} />
        <Kpi label="Still to pay" amount={d?.due} tone="out" v={d ? money(sym, d.due) : '…'} />
      </KpiStrip>
      <DataList label="Deliveries" rows={d?.rows ?? []} rowKey={(x) => x.id} onRow={(x) => window.open(`/print/grn/${x.id}`, '_blank')}
        empty={<EmptyState title={d ? 'Nothing received in this period' : 'Loading…'} />}
        columns={[
          { key: 'sup', header: 'Delivery', cell: (x) => (
            <span className="flex flex-col">
              <span className="flex items-center gap-1.5"><b>{x.supplier}</b>{num(x.due) > 0 && <Tag tone="danger">unpaid</Tag>}</span>
              <small className="font-normal text-faint">{stamp(x.when)}{x.invoice && ` · ${x.invoice}`}</small>
            </span>
          ) },
          { key: 'total', header: 'Total', num: true, cell: (x) => <b>{money(sym, x.total)}</b> },
          { key: 'paid', header: 'Paid', num: true, wide: true, cell: (x) => money(sym, x.paid) },
          { key: 'back', header: 'Sent back', num: true, wide: true, cell: (x) => (num(x.credited) > 0 ? money(sym, x.credited) : <span className="text-faint">—</span>) },
          { key: 'due', header: 'To pay', num: true, cell: (x) => <span className={cn(num(x.due) > 0 && 'text-danger')}>{money(sym, x.due)}</span> },
        ]} />
    </ReportPage>
  )
}
