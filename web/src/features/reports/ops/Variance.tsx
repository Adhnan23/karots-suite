import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { qtyText, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { periodText } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { varianceQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/recipe-variance')

/** Recipe variance (karots-pos /admin/reports/recipe-variance): what the
 * recipes say was used against what the stock actually moved. A yield is an
 * estimate, so some drift is normal; over 10% is worth a look. */
export function VariancePage() {
  const r = rangeOf(route.useSearch())
  const navigate = route.useNavigate()
  const d = useQuery({ ...varianceQuery(r), placeholderData: keepPreviousData }).data
  return (
    <ReportPage title="Recipe variance" sub="Ingredients the recipes say were used, against what actually left the shelf."
      range={r} onRange={(x) => void navigate({ search: (p) => ({ ...p, ...x }), replace: true })} caption={d && periodText(d.from, d.to)}
      exportUrl={`/api/v1/admin/reports/recipe-variance?${reportQS(r)}`}>
      <KpiStrip label="Headline" cols={3}>
        <Kpi label="Ingredients tracked" v={d ? String(d.rows.length) : '…'} />
        <Kpi label="Drifting over 10%" amount={d ? String(d.drifting) : '0'} tone="out" v={d ? String(d.drifting) : '…'} />
        <Kpi label="Within 10%" v={d ? String(d.rows.length - d.drifting) : '…'} className="max-md:col-span-2" />
      </KpiStrip>
      <DataList label="Ingredients" rows={d?.rows ?? []} rowKey={(x) => x.ingredient + x.unit}
        empty={<EmptyState title={d ? 'No recipe sales in this period' : 'Loading…'} />}
        columns={[
          { key: 'name', header: 'Ingredient', cell: (x) => <b>{x.ingredient}</b> },
          { key: 'exp', header: 'Recipes say', num: true, cell: (x) => `${qtyText(x.expected)} ${x.unit}` },
          { key: 'act', header: 'Actually used', num: true, cell: (x) => `${qtyText(x.actual)} ${x.unit}` },
          { key: 'diff', header: 'Difference', num: true, wide: true, cell: (x) => `${num(x.diff) > 0 ? '+' : ''}${qtyText(x.diff)}` },
          { key: 'drift', header: 'Drift', num: true, cell: (x) => <b className={cn(Math.abs(num(x.drift)) > 10 ? 'text-danger' : 'text-success')}>{num(x.drift) > 0 ? '+' : ''}{x.drift}%</b> },
        ]} />
    </ReportPage>
  )
}
