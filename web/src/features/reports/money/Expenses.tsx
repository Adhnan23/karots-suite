import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { BarCell } from '../kit/BarCell'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { pctText, periodText, share } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { expensesReportQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/expenses')

/** Expenses by category: where the running
 * costs went — the detail behind the P&L's expenses line. Tap one for its
 * expenses. */
export function ExpensesReportPage() {
  const s = route.useSearch()
  const r = rangeOf(s)
  const navigate = route.useNavigate()
  const go = useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...expensesReportQuery(r), placeholderData: keepPreviousData }).data
  const top = num(d?.rows[0]?.total ?? '0')
  const biggest = d?.rows[0]
  return (
    <ReportPage title="Expenses by category" sub="Where the running costs went — the detail behind the P&L’s expenses line."
      range={r} onRange={(x) => void navigate({ search: (p) => ({ ...p, ...x }), replace: true })} caption={d && periodText(d.from, d.to)}
      exportUrl={`/api/v1/admin/reports/expenses?${reportQS(r)}`}>
      <KpiStrip label="Headline" cols={3}>
        <Kpi label="↑ Spent" amount={d?.total} tone="out" v={d ? money(sym, d.total) : '…'} />
        <Kpi label="Expenses" v={d ? String(d.count) : '…'} sub={d && `${d.rows.length} categor${d.rows.length === 1 ? 'y' : 'ies'}`} />
        <Kpi label="Biggest" v={biggest ? biggest.category : '—'} sub={biggest && d && `${money(sym, biggest.total)} · ${pctText(share(biggest.total, d.total))}`} className="phone:col-span-2" />
      </KpiStrip>
      <DataList label="Categories" rows={d?.rows ?? []} rowKey={(x) => x.category}
        onRow={(x) => void go({ to: '/admin/expenses', search: { preset: s.preset, from: s.from, to: s.to, category: x.category } })}
        empty={<EmptyState title={d ? 'No expenses in this period' : 'Loading…'} />}
        columns={[
          { key: 'cat', header: 'Category', cell: (x) => <span className="flex flex-col"><b>{x.category}</b><small className="font-normal text-faint">{x.count} expense{x.count === 1 ? '' : 's'}</small></span> },
          { key: 'total', header: 'Spent', num: true, cell: (x) => <BarCell pct={top > 0 ? (num(x.total) / top) * 100 : null} tone="warning">{money(sym, x.total)}</BarCell> },
          { key: 'share', header: 'Share', num: true, cell: (x) => <span className="text-muted-foreground">{pctText(share(x.total, d?.total ?? '0'))}</span> },
        ]} />
    </ReportPage>
  )
}
