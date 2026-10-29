import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { BarCell } from '../kit/BarCell'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { pctText, periodText, share } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { spendQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/supplier-spend')

/** Spend by supplier: who you buy
 * the most from, net of goods sent back, and what's still to pay them. */
export function SpendPage() {
  const r = rangeOf(route.useSearch())
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...spendQuery(r), placeholderData: keepPreviousData }).data
  const top = num(d?.rows[0]?.spend ?? '0')
  return (
    <ReportPage title="Spend by supplier" sub="Who you buy the most from, after goods sent back."
      range={r} onRange={(x) => void navigate({ search: (p) => ({ ...p, ...x }), replace: true })} caption={d && periodText(d.from, d.to)}
      exportUrl={`/api/v1/admin/reports/supplier-spend?${reportQS(r)}`}>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label="Bought" amount={d?.spend} v={d ? money(sym, d.spend) : '…'} />
        <Kpi label="Deliveries" v={d ? String(d.orders) : '…'} />
        <Kpi label="↑ Paid" v={d ? money(sym, d.paid) : '…'} />
        <Kpi label="Still to pay" amount={d?.due} tone="out" v={d ? money(sym, d.due) : '…'} />
      </KpiStrip>
      <DataList label="Suppliers" rows={d?.rows ?? []} rowKey={(x) => x.supplierId}
        onRow={(x) => void navigate({ to: '/admin/suppliers/$id', params: { id: String(x.supplierId) } })}
        empty={<EmptyState title={d ? 'Nothing received in this period' : 'Loading…'} />}
        columns={[
          { key: 'name', header: 'Supplier', cell: (x) => <span className="flex flex-col"><b>{x.supplier}</b><small className="font-normal text-faint">{x.orders} deliver{x.orders === 1 ? 'y' : 'ies'}</small></span> },
          { key: 'spend', header: 'Bought', num: true, cell: (x) => <BarCell pct={top > 0 ? (num(x.spend) / top) * 100 : null}>{money(sym, x.spend)}</BarCell> },
          { key: 'share', header: 'Share', num: true, wide: true, cell: (x) => <span className="text-muted-foreground">{pctText(share(x.spend, d?.spend ?? '0'))}</span> },
          { key: 'paid', header: 'Paid', num: true, wide: true, cell: (x) => money(sym, x.paid) },
          { key: 'due', header: 'To pay', num: true, cell: (x) => <span className={cn(num(x.due) > 0 && 'text-danger')}>{money(sym, x.due)}</span> },
        ]} />
    </ReportPage>
  )
}
