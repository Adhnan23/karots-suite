import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { ChartTable } from '@/components/charts/ChartCard'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { useIsPhone } from '@/lib/useMedia'
import { cn } from '@/lib/utils'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { periodText, ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { peakHoursQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/peak-hours')
/** Monday first; values are Postgres DOW (0 = Sunday). */
const DAYS: [number, string][] = [[1, 'Mon'], [2, 'Tue'], [3, 'Wed'], [4, 'Thu'], [5, 'Fri'], [6, 'Sat'], [0, 'Sun']]
const LONG = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const hh = (h: number) => `${String(h).padStart(2, '0')}:00`

/** Peak hours (karots-pos /admin/reports/peak-hours): when the shop is busy,
 * by weekday and hour in shop time — for staffing and breaks. */
export function PeakHoursPage() {
  const s = route.useSearch()
  const r = rangeOf(s)
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const phone = useIsPhone()
  const d = useQuery({ ...peakHoursQuery(r), placeholderData: keepPreviousData }).data
  const grid = new Map(d?.cells.map((c) => [`${c.dow}:${c.hour}`, c]))
  const max = Math.max(1, ...(d?.cells.map((c) => c.count) ?? []))
  const hours = d?.cells.length ? d.cells.map((c) => c.hour) : []
  // Opening hours at least (08–20), widened to any sale outside them.
  const lo = Math.min(8, ...hours), hi = Math.max(20, ...hours)
  const span = Array.from({ length: hi - lo + 1 }, (_, i) => lo + i)
  const busiest = d?.cells.reduce<(typeof d.cells)[number] | null>((b, c) => (!b || c.count > b.count ? c : b), null)
  const byDay = DAYS.map(([dow, label]) => {
    const cells = d?.cells.filter((c) => c.dow === dow) ?? []
    const top = cells.reduce<(typeof cells)[number] | null>((b, c) => (!b || c.count > b.count ? c : b), null)
    return { dow, label, count: cells.reduce((n, c) => n + c.count, 0), revenue: cells.reduce((n, c) => n + num(c.revenue), 0), top }
  })
  const bestDay = byDay.reduce((b, x) => (x.count > b.count ? x : b), byDay[0])

  return (
    <ReportPage title="Peak hours" sub="When the shop is busy, by day of the week and hour — plan staff and breaks around it."
      range={r} onRange={(x) => void navigate({ search: (p) => ({ ...p, ...x }), replace: true })} caption={d && periodText(d.from, d.to)}
      exportUrl={`/api/v1/admin/reports/peak-hours?${reportQS(r)}`}>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label="Sales" v={d ? String(d.count) : '…'} />
        <Kpi label="Revenue" amount={d?.revenue} v={d ? money(sym, d.revenue) : '…'} />
        <Kpi label="Busiest hour" v={busiest ? `${LONG[busiest.dow].slice(0, 3)} ${hh(busiest.hour)}` : '—'} sub={busiest && `${busiest.count} sales`} />
        <Kpi label="Busiest day" v={bestDay?.count ? LONG[bestDay.dow] : '—'} sub={bestDay?.count ? `${bestDay.count} sales` : undefined} />
      </KpiStrip>
      <section aria-label="Sales by day and hour" className={cn(SURFACE, 'p-4')}>
        <h2 className="mb-3 font-display text-base font-bold">{phone ? 'By day' : 'Sales by day and hour'}</h2>
        {!d?.cells.length ? <EmptyState title={d ? 'No sales in this period' : 'Loading…'} />
          : phone ? (
            <ChartTable head={['Day', 'Sales', 'Busiest', 'Revenue']}
              rows={byDay.filter((x) => x.count).map((x) => [x.label, x.count, x.top ? hh(x.top.hour) : '—', money(sym, x.revenue)])} />
          ) : (
            <div className="overflow-x-auto">
              <table className="border-separate border-spacing-[3px] text-[12px]">
                <thead><tr><th />{span.map((h) => <th key={h} className="font-semibold text-faint">{String(h).padStart(2, '0')}</th>)}<th className="pl-2 text-right font-semibold text-faint">Sales</th></tr></thead>
                <tbody>
                  {byDay.map((x) => (
                    <tr key={x.dow}>
                      <th className="pr-2 text-left font-semibold text-muted-foreground">{x.label}</th>
                      {span.map((h) => {
                        const c = grid.get(`${x.dow}:${h}`)
                        const pct = c ? Math.round(15 + (c.count / max) * 55) : 0
                        return (
                          <td key={h} title={c ? `${LONG[x.dow]} ${hh(h)} — ${c.count} sales, ${money(sym, c.revenue)}` : undefined}
                            className="h-9 w-11 rounded-[6px] text-center font-semibold tabular-nums"
                            style={{ background: c ? `color-mix(in srgb, var(--chart-1) ${pct}%, var(--card))` : 'var(--muted)' }}>
                            {c?.count ?? ''}
                          </td>
                        )
                      })}
                      <td className="pl-2 text-right font-bold tabular-nums">{x.count || ''}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
      </section>
    </ReportPage>
  )
}
