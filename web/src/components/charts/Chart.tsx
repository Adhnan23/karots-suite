import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import * as echarts from 'echarts/core'
import type { EChartsCoreOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { useEffect, useRef, useState } from 'react'

echarts.use([LineChart, BarChart, PieChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

/** The palette's chart tokens, read at render time (theme.md: charts follow the
 * active palette and mode; series use --chart-1..4 in that fixed order). */
export type ChartTheme = { series: string[]; fg: string; muted: string; faint: string; border: string; card: string; font: string }

function readTheme(): ChartTheme {
  const css = getComputedStyle(document.documentElement)
  const v = (n: string) => css.getPropertyValue(n).trim()
  return {
    series: [1, 2, 3, 4].map((i) => v(`--chart-${i}`)), fg: v('--foreground'), muted: v('--muted-foreground'), faint: v('--faint'),
    border: v('--border'), card: v('--popover') || v('--card'), font: getComputedStyle(document.body).fontFamily,
  }
}

/** Re-read the theme when the palette or mode changes (data-palette /
 * data-theme on <html>, or the OS scheme under "system"). */
function useChartTheme(): ChartTheme {
  const [t, setT] = useState(readTheme)
  useEffect(() => {
    const again = () => setT(readTheme())
    const mo = new MutationObserver(again)
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'data-palette', 'class'] })
    const mq = matchMedia('(prefers-color-scheme: dark)')
    mq.addEventListener('change', again)
    return () => { mo.disconnect(); mq.removeEventListener('change', again) }
  }, [])
  return t
}

/** One ECharts chart. Loaded lazily (only chart routes pull echarts in);
 * `option` builds the chart from the current theme. */
export default function Chart({ option, label, height = 260 }: { option: (t: ChartTheme) => EChartsCoreOption; label: string; height?: number }) {
  const el = useRef<HTMLDivElement>(null)
  const chart = useRef<echarts.ECharts | null>(null)
  const theme = useChartTheme()
  useEffect(() => {
    const c = echarts.init(el.current!, undefined, { renderer: 'canvas' })
    chart.current = c
    const ro = new ResizeObserver(() => c.resize())
    ro.observe(el.current!)
    return () => { ro.disconnect(); c.dispose(); chart.current = null }
  }, [])
  useEffect(() => {
    chart.current?.setOption({ textStyle: { fontFamily: theme.font, color: theme.muted }, animationDuration: 300, ...option(theme) }, true)
  }, [option, theme])
  return <div ref={el} role="img" aria-label={label} style={{ height }} className="w-full" />
}
