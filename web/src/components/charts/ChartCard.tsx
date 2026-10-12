import type { EChartsCoreOption } from 'echarts/core'
import { lazy, type ReactNode, Suspense, useState } from 'react'
import { SURFACE } from '@/components/admin/PageHead'
import { Seg } from '@/features/till/bits'
import { useIsPhone } from '@/lib/useMedia'
import { cn } from '@/lib/utils'
import type { ChartTheme } from './Chart'

const Chart = lazy(() => import('./Chart'))

/** A chart with its table: phones get the table, desktops the chart with a
 * switch to the table (the chart's accessible twin). */
export function ChartCard({ title, sub, option, label, table, height, empty, className }: {
  title: string; sub?: string; option: (t: ChartTheme) => EChartsCoreOption; label: string; table: ReactNode; height?: number; empty?: boolean; className?: string
}) {
  const phone = useIsPhone()
  const [view, setView] = useState<'chart' | 'table' | null>(null)
  const v = view ?? (phone ? 'table' : 'chart')
  return (
    <section aria-label={title} className={cn(SURFACE, 'flex min-w-0 flex-col gap-3 p-4', className)}>
      <header className="flex items-start justify-between gap-2">
        <div><h2 className="font-display text-base font-bold">{title}</h2>{sub && <p className="text-[12px] text-muted-foreground">{sub}</p>}</div>
        {!empty && <Seg label={`${title} view`} value={v} onChange={setView} options={[['chart', 'Chart'], ['table', 'Table']]} />}
      </header>
      {empty ? <p className="py-10 text-center text-sm text-faint">Nothing in this range.</p>
        : v === 'chart' ? <Suspense fallback={<div style={{ height: height ?? 260 }} />}><Chart option={option} label={label} height={height} /></Suspense> : table}
    </section>
  )
}

/** The compact table a chart card falls back to. */
export function ChartTable({ head, rows }: { head: string[]; rows: ReactNode[][] }) {
  return (
    <div className="max-h-80 overflow-auto">
      <table className="w-full text-sm">
        <thead><tr className="border-b border-border text-left text-[12px] text-faint">{head.map((h, i) => <th key={h} className={cn('py-1.5 font-semibold', i > 0 && 'pl-3 text-right')}>{h}</th>)}</tr></thead>
        <tbody>{rows.map((r, i) => <tr key={i} className="border-b border-border last:border-0">{r.map((c, j) => <td key={j} className={cn('py-1.5', j > 0 && 'whitespace-nowrap pl-3 text-right tabular-nums')}>{c}</td>)}</tr>)}</tbody>
      </table>
    </div>
  )
}
