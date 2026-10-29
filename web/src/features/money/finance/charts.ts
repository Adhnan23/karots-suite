import type { EChartsCoreOption } from 'echarts/core'
import type { FinanceCategory, FinancePoint, FinanceSlice } from '@/api/types.gen'
import type { ChartTheme } from '@/components/charts/Chart'
import { money, num } from '@/lib/money'

export const axis = (t: ChartTheme) => ({
  axisLine: { lineStyle: { color: t.border } }, axisTick: { show: false }, axisLabel: { color: t.faint, fontSize: 11 },
  splitLine: { lineStyle: { color: t.border, opacity: 0.6 } },
})
export const tip = (t: ChartTheme) => ({ backgroundColor: t.card, borderColor: t.border, textStyle: { color: t.fg, fontSize: 12 }, confine: true })
export const short = (sym: string) => (v: number) => (Math.abs(v) >= 1000 ? `${sym} ${(v / 1000).toFixed(v % 1000 ? 1 : 0)}k` : `${sym} ${v}`)
export const methodLabel = (m: string) => ({ cash: 'Cash', card: 'Card', online: 'Online', cheque: 'Cheque', credit: 'Credit' })[m] ?? m

/** Revenue and profit over the range — one money axis, two 2px lines, a
 * crosshair tooltip; the legend names both and the last points are labelled. */
export function trendOption(rows: FinancePoint[], sym: string, grain: string) {
  return (t: ChartTheme): EChartsCoreOption => {
    const fmt = (d: string) => new Date(`${d}T00:00`).toLocaleDateString([], grain === 'month' ? { month: 'short', year: '2-digit' } : { day: 'numeric', month: 'short' })
    const line = (name: string, key: 'revenue' | 'profit', color: string) => ({
      name, type: 'line', data: rows.map((r) => num(r[key])), smooth: false, symbol: 'circle', symbolSize: 8, showSymbol: rows.length <= 12,
      lineStyle: { width: 2, color }, itemStyle: { color, borderColor: t.card, borderWidth: 2 },
      endLabel: { show: rows.length > 1, color: t.muted, fontSize: 11, formatter: () => name },
    })
    return {
      grid: { left: 8, right: 56, top: 36, bottom: 8, containLabel: true },
      legend: { top: 0, left: 0, icon: 'roundRect', itemWidth: 12, itemHeight: 4, textStyle: { color: t.muted } },
      tooltip: { trigger: 'axis', axisPointer: { type: 'line', lineStyle: { color: t.faint } }, ...tip(t), valueFormatter: (v: unknown) => money(sym, v) },
      xAxis: { type: 'category', boundaryGap: false, data: rows.map((r) => fmt(r.day)), ...axis(t), splitLine: { show: false } },
      yAxis: { type: 'value', ...axis(t), axisLine: { show: false }, axisLabel: { ...axis(t).axisLabel, formatter: short(sym) } },
      series: [line('Revenue', 'revenue', t.series[0]), line('Profit', 'profit', t.series[1])],
    }
  }
}

/** How the takings were paid — a donut with a 2px surface gap between slices
 * and each slice labelled (identity never by colour alone). */
export function paymentOption(rows: FinanceSlice[], sym: string) {
  return (t: ChartTheme): EChartsCoreOption => ({
    tooltip: { trigger: 'item', ...tip(t), formatter: (p: { name: string; value: number; percent: number }) => `${p.name}<br/><b>${money(sym, p.value)}</b> · ${p.percent}%` },
    series: [{
      type: 'pie', radius: ['52%', '78%'], center: ['50%', '52%'], avoidLabelOverlap: true,
      itemStyle: { borderColor: t.card, borderWidth: 2, borderRadius: 4 },
      label: { color: t.fg, fontSize: 12, formatter: '{b}\n{d}%' }, labelLine: { lineStyle: { color: t.border } },
      data: rows.map((r, i) => ({ name: methodLabel(r.method), value: num(r.total), itemStyle: { color: t.series[i % 4] } })),
    }],
  })
}

/** Profit by category, highest first — one hue, bars rounded at the data end,
 * value labels at the bar end. */
export function categoryOption(rows: FinanceCategory[], sym: string) {
  return (t: ChartTheme): EChartsCoreOption => {
    const top = [...rows].sort((a, b) => num(b.profit) - num(a.profit)).slice(0, 10).reverse()
    return {
      grid: { left: 8, right: 72, top: 8, bottom: 8, containLabel: true },
      tooltip: { trigger: 'item', ...tip(t), formatter: (p: { dataIndex: number }) => {
        const r = top[p.dataIndex]
        return `${r.category}<br/>Profit <b>${money(sym, r.profit)}</b><br/>Revenue ${money(sym, r.revenue)}`
      } },
      xAxis: { type: 'value', ...axis(t), axisLabel: { ...axis(t).axisLabel, formatter: short(sym) } },
      yAxis: { type: 'category', data: top.map((r) => r.category), ...axis(t), axisLabel: { color: t.muted, fontSize: 12, width: 120, overflow: 'truncate' } },
      series: [{
        type: 'bar', data: top.map((r) => num(r.profit)), barMaxWidth: 18, barCategoryGap: '30%',
        itemStyle: { color: t.series[0], borderRadius: [0, 4, 4, 0] },
        label: { show: true, position: 'right', color: t.muted, fontSize: 11, formatter: (p: { value: number }) => short(sym)(p.value) },
      }],
    }
  }
}
