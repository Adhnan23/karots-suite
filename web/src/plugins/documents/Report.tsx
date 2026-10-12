import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import type { EChartsCoreOption } from 'echarts/core'
import { Download } from 'lucide-react'
import { useState } from 'react'
import { ChartCard, ChartTable } from '@/components/charts/ChartCard'
import type { ChartTheme } from '@/components/charts/Chart'
import { Button } from '@/components/ui/button'
import type { Range } from '@/features/counter/range'
import { axis, tip } from '@/features/money/finance/charts'
import { Kpi, KpiStrip } from '@/features/reports/kit/Kpi'
import { ReportPage } from '@/features/reports/kit/ReportPage'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { reportQuery } from './api'
import { optionsText } from './labels'
import type { ReportRow } from './types'

const TOP = 10
const label = (r: ReportRow) => [r.name, r.earlier ? 'earlier jobs' : optionsText(r.size, r.color, r.double)].filter(Boolean).join(' — ')

function chart(rows: ReportRow[]) {
  const top = rows.slice(0, TOP).reverse() // horizontal bars read top-down
  return (t: ChartTheme): EChartsCoreOption => ({
    grid: { left: 8, right: 24, top: 8, bottom: 8, containLabel: true },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, ...tip(t) },
    xAxis: { type: 'value', minInterval: 1, ...axis(t) },
    yAxis: { type: 'category', data: top.map(label), ...axis(t), splitLine: { show: false }, axisLabel: { ...axis(t).axisLabel, width: 150, overflow: 'truncate' } },
    series: [{ name: 'Copies', type: 'bar', data: top.map((r) => num(r.copies)), barMaxWidth: 22, itemStyle: { color: t.series[0], borderRadius: [0, 4, 4, 0] },
      label: { show: true, position: 'right', color: t.muted } }],
  })
}

function downloadCsv(rows: ReportRow[], range: Range) {
  const q = (s: string) => `"${s.replace(/"/g, '""')}"`
  const body = [['Service', 'Options', 'Jobs', 'Copies', 'Sheets', 'Sales'],
    ...rows.map((r) => [q(r.name), q(r.earlier ? 'earlier jobs' : optionsText(r.size, r.color, r.double)), String(r.jobs), r.copies, r.sheets, r.revenue])]
    .map((r) => r.join(',')).join('\n')
  const a = document.createElement('a')
  a.href = URL.createObjectURL(new Blob([body], { type: 'text/csv' }))
  a.download = `print-copy-${range.from || range.preset || 'report'}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}

/** Reports → Print & Copy: copies, sheets and sales per service × size × colour × side. */
export function PrintReport() {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [range, setRange] = useState<Range>({ preset: 'this-month', from: '', to: '' })
  const d = useQuery(reportQuery(range)).data
  const rows = d?.rows ?? []
  const table = (
    <ChartTable head={['Job', 'Copies', 'Sheets', 'Sales']}
      rows={rows.map((r) => [label(r), num(r.copies), num(r.sheets), money(sym, r.revenue)])} />
  )
  return (
    <ReportPage title="Print & Copy" sub="Copies, sheets and sales per job type." range={range} onRange={setRange}
      filters={rows.length > 0 && <Button variant="outline" className="h-10 gap-1.5 px-4" onClick={() => downloadCsv(rows, range)}><Download size={16} /> CSV</Button>}
      caption="Copies and sales leave out returned copies; sheets count everything printed. The money view of each service is Reports → Service profit.">
      {d && (
        <KpiStrip label="Print & Copy totals" cols={4}>
          <Kpi label="Copies sold" v={String(num(d.copies))} sub={`${d.jobs} job${d.jobs === 1 ? '' : 's'}`} />
          <Kpi label="Sheets used" v={String(num(d.sheets))} />
          <Kpi label="Sales" v={money(sym, d.revenue)} />
          <Kpi amount={d.ownUseCost} label="Shop use" v={money(sym, d.ownUseCost)} tone="out" sub={`${num(d.ownUseSheets)} sheets · ${d.ownUseJobs} job${d.ownUseJobs === 1 ? '' : 's'}`} />
        </KpiStrip>
      )}
      <ChartCard title="Copies" sub={rows.length > TOP ? `Top ${TOP}` : undefined} label="Copies per job type" option={chart(rows)} table={table}
        empty={!!d && rows.length === 0} height={Math.max(180, Math.min(rows.length, TOP) * 34 + 24)} />
    </ReportPage>
  )
}
