import type { EChartsCoreOption } from 'echarts/core'
import type { ProductPeriod } from '@/api/types.gen'
import type { ChartTheme } from '@/components/charts/Chart'
import { axis, tip } from '@/features/money/finance/charts'
import { num } from '@/lib/money'

/** One product's units per period, with the same period last year beside it
 * (karots-pos Product Sales overlay) — paired bars, this year in series 1. */
export function productPeriodOption(rows: ProductPeriod[], grain: string) {
  return (t: ChartTheme): EChartsCoreOption => {
    const fmt = (d: string) => new Date(`${d}T00:00`).toLocaleDateString([], grain === 'month' ? { month: 'short', year: '2-digit' } : { day: 'numeric', month: 'short' })
    const bar = (name: string, key: 'qty' | 'lastYearQty', color: string) => ({
      name, type: 'bar', data: rows.map((r) => num(r[key])), barMaxWidth: 22, itemStyle: { color, borderRadius: [4, 4, 0, 0] },
    })
    return {
      grid: { left: 8, right: 8, top: 36, bottom: 8, containLabel: true },
      legend: { top: 0, left: 0, icon: 'roundRect', itemWidth: 12, itemHeight: 8, textStyle: { color: t.muted } },
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, ...tip(t) },
      xAxis: { type: 'category', data: rows.map((r) => fmt(r.day)), ...axis(t), splitLine: { show: false } },
      yAxis: { type: 'value', minInterval: 1, ...axis(t), axisLine: { show: false } },
      series: [bar('This period', 'qty', t.series[0]), bar('Same time last year', 'lastYearQty', t.series[2])],
    }
  }
}
