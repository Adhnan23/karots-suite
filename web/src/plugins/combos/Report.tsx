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
import type { ReportRow } from './types'

const TOP = 10

function chart(rows: ReportRow[]) {
  const top = rows.slice(0, TOP).reverse() // horizontal bars read top-down
  return (t: ChartTheme): EChartsCoreOption => ({
    grid: { left: 8, right: 24, top: 8, bottom: 8, containLabel: true },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, ...tip(t) },
    xAxis: { type: 'value', minInterval: 1, ...axis(t) },
    yAxis: { type: 'category', data: top.map((r) => r.name), ...axis(t), splitLine: { show: false } },
    series: [{ name: 'Sets sold', type: 'bar', data: top.map((r) => num(r.sets)), barMaxWidth: 22, itemStyle: { color: t.series[0], borderRadius: [0, 4, 4, 0] },
      label: { show: true, position: 'right', color: t.muted } }],
  })
}

function downloadCsv(rows: ReportRow[], range: Range) {
  const q = (s: string) => `"${s.replace(/"/g, '""')}"`
  const body = [['Combo', 'Sets', 'Sales', 'Discount given', 'Bills'], ...rows.map((r) => [q(r.name), r.sets, r.net, r.discount, String(r.bills)])]
    .map((r) => r.join(',')).join('\n')
  const a = document.createElement('a')
  a.href = URL.createObjectURL(new Blob([body], { type: 'text/csv' }))
  a.download = `combos-${range.from || range.preset || 'report'}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}

/** Reports → Combos: sets sold, sales and the discount each combo gave away. */
export function ComboReport() {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [range, setRange] = useState<Range>({ preset: 'this-month', from: '', to: '' })
  const data = useQuery(reportQuery(range)).data
  const rows = data?.rows ?? []
  const t = data?.totals
  const table = (
    <ChartTable head={['Combo', 'Sets', 'Sales', 'Discount', 'Bills']}
      rows={[...rows.map((r) => [r.name, num(r.sets), money(sym, r.net), money(sym, r.discount), r.bills]),
        ...(t && rows.length > 1 ? [[<b key="t">Total</b>, <b key="s">{num(t.sets)}</b>, <b key="n">{money(sym, t.net)}</b>, <b key="d">{money(sym, t.discount)}</b>, <b key="b">{t.bills}</b>]] : [])]} />
  )
  return (
    <ReportPage title="Combos" sub="What each combo sold and what it gave away." range={range} onRange={setRange}
      filters={rows.length > 0 && <Button variant="outline" className="h-10 gap-1.5 px-4" onClick={() => downloadCsv(rows, range)}><Download size={16} /> CSV</Button>}
      caption="Counts sales since Combos was added. A returned combo still counts as sold.">
      {t && (
        <KpiStrip label="Combo totals" cols={3}>
          <Kpi label="Sets sold" v={String(num(t.sets))} />
          <Kpi label="Combo sales" v={money(sym, t.net)} />
          <Kpi amount={t.discount} label="Discount given" v={money(sym, t.discount)} tone="out" sub={num(t.net) > 0 ? `${((num(t.discount) / (num(t.net) + num(t.discount))) * 100).toFixed(1)}% of full price` : undefined} />
        </KpiStrip>
      )}
      <ChartCard title="Sets sold" sub={rows.length > TOP ? `Top ${TOP}` : undefined} label="Sets sold per combo" option={chart(rows)} table={table}
        empty={!!data && rows.length === 0} height={Math.max(180, Math.min(rows.length, TOP) * 34 + 24)} />
    </ReportPage>
  )
}
