import type { EChartsCoreOption } from 'echarts/core'
import type { FinancePoint } from '@/api/types.gen'
import type { ChartTheme } from '@/components/charts/Chart'
import { axis, short, tip } from '@/features/money/finance/charts'
import { money, num } from '@/lib/money'

/** This period's revenue against the period before it (the mockup's trend):
 * a solid line with a soft area for now, a dashed one for before. The tooltip
 * names both days and the change. */
export function compareOption(cur: FinancePoint[], prev: FinancePoint[], sym: string, grain: string) {
  return (t: ChartTheme): EChartsCoreOption => {
    const fmt = (d: string, o: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' }) => new Date(`${d}T00:00`).toLocaleDateString([], o)
    return {
      grid: { left: 8, right: 16, top: 36, bottom: 8, containLabel: true },
      legend: { top: 0, left: 0, icon: 'roundRect', itemWidth: 14, itemHeight: 3, textStyle: { color: t.muted } },
      tooltip: {
        trigger: 'axis', ...tip(t), axisPointer: { type: 'line', lineStyle: { color: t.faint } },
        formatter: (ps: { dataIndex: number }[]) => {
          const i = ps[0]?.dataIndex ?? 0
          const c = num(cur[i]?.revenue ?? '0'), p = num(prev[i]?.revenue ?? '0')
          const d = p ? Math.round((c / p - 1) * 100) : null
          const when = grain === 'week' ? `Week of ${fmt(cur[i].day)}` : fmt(cur[i].day, { weekday: 'short', day: 'numeric', month: 'short' })
          return `<b>${when}</b><br/>${money(sym, c)}${d === null ? '' : ` &nbsp;${d >= 0 ? '▲' : '▼'} ${Math.abs(d)}%`}<br/><span style="opacity:.7">Before: ${money(sym, p)}</span>`
        },
      },
      xAxis: { type: 'category', boundaryGap: false, data: cur.map((r) => fmt(r.day)), ...axis(t), splitLine: { show: false } },
      yAxis: { type: 'value', ...axis(t), axisLine: { show: false }, axisLabel: { ...axis(t).axisLabel, formatter: short(sym) } },
      series: [
        { name: 'This period', type: 'line', data: cur.map((r) => num(r.revenue)), symbol: 'circle', symbolSize: 7, showSymbol: cur.length <= 14,
          lineStyle: { width: 2.5, color: t.series[0] }, itemStyle: { color: t.series[0], borderColor: t.card, borderWidth: 2 },
          areaStyle: { color: t.series[0], opacity: 0.12 } },
        { name: 'The period before', type: 'line', data: prev.map((r) => num(r.revenue)), symbol: 'none',
          lineStyle: { width: 1.6, type: 'dashed', color: t.faint }, itemStyle: { color: t.faint } },
      ],
    }
  }
}
