import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'
import { AlertTriangle, ArrowDownRight, ArrowUpRight, Boxes, CalendarClock, ChevronRight, ClipboardPen, Store, UserRoundX } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { type ReactNode, useMemo, useState } from 'react'
import type { Dashboard as D } from '@/api/types.gen'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { ChartCard, ChartTable } from '@/components/charts/ChartCard'
import { methodLabel } from '@/features/money/finance/charts'
import { stamp } from '@/features/money/format'
import { delta, pctText } from '@/features/reports/kit/numbers'
import { SaleSheet } from '@/features/sales/SaleSheet'
import { Seg } from '@/features/till/bits'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { compareOption } from './charts'
import { dashboardChartsQuery, dashboardQuery } from './queries'
import { Spark } from './Spark'

const route = getRouteApi('/_authed/admin/')
const greeting = (h: number) => (h < 12 ? 'Good morning' : h < 17 ? 'Good afternoon' : 'Good evening')

/** One headline figure with its change against yesterday and the last 14 days
 * as a sparkline (the mockup's KPI tile). */
function Tile({ label, value, change, fallIsGood, sub, spark, to }: {
  label: string; value: string; change?: number | null; fallIsGood?: boolean; sub: ReactNode; spark?: number[]; to?: string
}) {
  const up = (change ?? 0) >= 0
  const good = up !== !!fallIsGood
  const body = (
    <>
      <span className="flex items-center justify-between gap-2 text-[12.5px] font-semibold text-muted-foreground">
        {label}
        {change != null && (
          <span className={cn('inline-flex items-center gap-0.5 rounded-full px-1.5 text-[11.5px] font-bold', good ? 'bg-[color-mix(in_srgb,var(--success)_14%,transparent)] text-success' : 'bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] text-danger')}>
            {up ? <ArrowUpRight size={12} aria-hidden /> : <ArrowDownRight size={12} aria-hidden />}<span className="sr-only">{up ? 'up' : 'down'}</span>{pctText(Math.abs(change))}
          </span>
        )}
      </span>
      <span className="flex items-end justify-between gap-2">
        <b className="font-display text-[20px] leading-tight tabular-nums whitespace-nowrap md:text-2xl">{value}</b>
        {spark && <Spark values={spark} className="h-auto w-14 min-w-0 shrink text-brand md:w-[84px]" />}
      </span>
      <small className="text-[12px] text-faint">{sub}</small>
    </>
  )
  const cls = cn(SURFACE, 'flex min-w-[78%] snap-start flex-col gap-1 overflow-hidden px-4 py-3.5 md:min-w-0')
  return to ? <Link to={to} className={cn(cls, 'transition-colors hover:border-selected')}>{body}</Link> : <div className={cls}>{body}</div>
}

type Att = { tone: 'crit' | 'warn' | 'info'; icon: LucideIcon; tag: string; text: string; to: string; go: string }

function attention(d: D, sym: string): Att[] {
  const a = d.attention
  const n = (k: number, one: string, many: string) => `${k} ${k === 1 ? one : many}`
  const out: Att[] = []
  if (a.overLimit) out.push({ tone: 'crit', icon: UserRoundX, tag: 'Over limit', text: `${n(a.overLimit, 'customer owes', 'customers owe')} past their credit limit (${money(sym, a.overLimitSum)})`, to: '/admin/reports/customer-dues', go: 'See dues' })
  if (a.expired) out.push({ tone: 'crit', icon: CalendarClock, tag: 'Expired', text: `${n(a.expired, 'lot is', 'lots are')} past its date and still on the shelf`, to: '/admin/stock/batches', go: 'Write off' })
  if (a.expiring) out.push({ tone: 'warn', icon: CalendarClock, tag: 'Expiring', text: `${n(a.expiring, 'lot expires', 'lots expire')} within 30 days`, to: '/admin/stock/batches', go: 'Sell first' })
  if (a.lowStock) out.push({ tone: 'warn', icon: Boxes, tag: 'Low stock', text: `${n(a.lowStock, 'product is', 'products are')} at or under the reorder level`, to: '/admin/reorder', go: 'Reorder' })
  if (a.review) out.push({ tone: 'info', icon: ClipboardPen, tag: 'Review', text: `${n(a.review, 'quick-added item needs', 'quick-added items need')} details`, to: '/admin/products/review', go: 'Review' })
  if (!d.openTills) out.push({ tone: 'info', icon: Store, tag: 'Till', text: 'No till is open', to: '/till', go: 'Open the till' })
  return out
}

/** The admin home (karots-pos Dashboard, in the approved mockup's shape):
 * today against yesterday, the trend against the period before, how people
 * paid, the top sellers and what needs attention. */
export function Dashboard() {
  const { me } = route.useRouteContext()
  const s = route.useSearch()
  const days = s.days ?? 14
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery(dashboardQuery).data
  const ch = useQuery({ ...dashboardChartsQuery(days), placeholderData: keepPreviousData }).data
  const [open, setOpen] = useState<number | null>(null)
  const trend = useMemo(() => ch && compareOption(ch.cur, ch.prev, sym, ch.grain), [ch, sym])
  const now = new Date()
  const paidTotal = d?.paid.reduce((n, p) => n + num(p.total), 0) ?? 0
  const topMax = num(ch?.top[0]?.revenue ?? '0')
  const att = d ? attention(d, sym) : []
  const card = (title: string, children: ReactNode, aside?: ReactNode) => (
    <section aria-label={title} className={cn(SURFACE, 'flex min-w-0 flex-col gap-3 p-4')}>
      <header className="flex items-center justify-between gap-2"><h2 className="font-display text-base font-bold">{title}</h2>{aside}</header>
      {children}
    </section>
  )
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="font-display text-[26px] leading-tight font-bold tracking-tight md:text-[28px]">{greeting(now.getHours())}, {me.user.name.split(' ')[0]}</h1>
          <p className="text-muted-foreground">{now.toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long' })} · {me.shopName}</p>
        </div>
        <Seg label="Chart range" value={String(days)} onChange={(v) => void navigate({ search: { days: v === '14' ? undefined : Number(v) }, replace: true })}
          options={[['7', '7 days'], ['14', '14 days'], ['30', '30 days'], ['90', '90 days']]} />
      </div>

      <div aria-label="Today" role="group" className="-mx-4 flex snap-x snap-mandatory scroll-px-4 gap-2.5 overflow-x-auto px-4 pb-1 [scrollbar-width:none] md:mx-0 md:grid md:grid-cols-4 md:overflow-visible md:px-0">
        <Tile label="Sales today" value={d ? money(sym, d.today.revenue) : '…'} change={d ? delta(d.today.revenue, d.yesterday.revenue) : null}
          sub={d ? `${d.today.count} sale${d.today.count === 1 ? '' : 's'} · avg ${money(sym, d.today.avgBasket)} · vs yesterday` : ''} spark={d?.spark.map((p) => num(p.revenue))} to="/admin/reports/sales?preset=today" />
        <Tile label="Gross profit" value={d ? money(sym, d.today.profit) : '…'} change={d ? delta(d.today.profit, d.yesterday.profit) : null}
          sub={d ? `${d.today.margin}% margin · vs yesterday` : ''} spark={d?.spark.map((p) => num(p.profit))} to="/admin/finance?preset=today" />
        <Tile label="Cash in the tills" value={d ? money(sym, d.tillCash) : '…'} sub={d ? (d.openTills ? `${d.openTills} till${d.openTills === 1 ? '' : 's'} open · expected now` : 'No till open') : ''} to="/admin/tills?open=1" />
        <Tile label="Owed to you" value={d ? money(sym, d.owed) : '…'} sub={d ? `${d.owing} customer${d.owing === 1 ? '' : 's'} on credit` : ''} to="/admin/reports/customer-dues" />
      </div>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        {ch && trend && (
          <ChartCard title="Sales, this period against the one before" sub={ch.grain === 'week' ? 'By week' : 'By day'} option={trend} label="Revenue this period and the period before"
            empty={!ch.cur.some((p) => num(p.revenue)) && !ch.prev.some((p) => num(p.revenue))}
            table={<ChartTable head={['Period', 'This period', 'Before']} rows={ch.cur.map((p, i) => [p.day, money(sym, p.revenue), money(sym, ch.prev[i]?.revenue ?? '0')]).reverse()} />} />
        )}
        {card('How customers paid today', d && d.paid.length ? (
          <ul className="flex flex-col gap-2.5 text-sm">
            {d.paid.map((p) => (
              <li key={p.method} className="grid grid-cols-[74px_minmax(0,1fr)_auto] items-center gap-2.5">
                <span className="flex flex-col leading-tight"><b>{methodLabel(p.method)}</b><small className="text-faint">{pctText(paidTotal ? (num(p.total) / paidTotal) * 100 : null)}</small></span>
                <span className="h-2.5 overflow-hidden rounded-r bg-muted"><span className="block h-full rounded-r bg-brand" style={{ width: `${paidTotal ? (num(p.total) / Math.max(...d.paid.map((x) => num(x.total)))) * 100 : 0}%` }} /></span>
                <b className="text-right tabular-nums">{money(sym, p.total)}</b>
              </li>
            ))}
          </ul>
        ) : <p className="py-8 text-center text-sm text-faint">{d ? 'Nothing taken yet today.' : 'Loading…'}</p>)}
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        {card(`Top sellers, last ${days} days`, ch && ch.top.length ? (
          <ol className="flex flex-col divide-y divide-border text-sm">
            {ch.top.map((p, i) => (
              <li key={p.productId}>
                <Link to="/admin/reports/product-sales" search={{ product: String(p.productId) }} className="grid grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-2.5 py-2 hover:text-brand">
                  <span className="font-bold text-faint">{i + 1}</span>
                  <span className="min-w-0"><span className="block truncate">{p.name}</span><span className="mt-1 block h-1.5 rounded-r bg-brand/80" style={{ width: `${topMax ? (num(p.revenue) / topMax) * 100 : 0}%` }} /></span>
                  <b className="tabular-nums">{money(sym, p.revenue)}</b>
                </Link>
              </li>
            ))}
          </ol>
        ) : <p className="py-8 text-center text-sm text-faint">{ch ? 'Nothing sold in this range.' : 'Loading…'}</p>,
        <Link to="/admin/reports/top-products" className="text-[13px] font-semibold text-muted-foreground hover:text-foreground">All →</Link>)}
        {card('Needs your attention', att.length ? (
          <ul className="flex flex-col gap-2">
            {att.map((x) => (
              <li key={x.tag + x.text}>
                <Link to={x.to} className="flex items-center gap-3 rounded-[var(--radius-sm)] bg-muted px-3 py-2.5 text-[13.5px] hover:bg-[color-mix(in_srgb,var(--muted)_70%,var(--foreground)_6%)]">
                  <span className={cn('inline-flex flex-none items-center gap-1 rounded-full px-2 py-0.5 text-[11.5px] font-extrabold',
                    x.tone === 'crit' ? 'bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] text-danger' : x.tone === 'warn' ? 'bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] text-warning' : 'bg-border text-muted-foreground')}>
                    {x.tone === 'crit' ? <AlertTriangle size={13} /> : <x.icon size={13} />}{x.tag}
                  </span>
                  <span className="min-w-0 flex-1">{x.text}</span>
                  <b className="hidden whitespace-nowrap md:inline">{x.go}</b><ChevronRight size={16} className="flex-none text-faint" />
                </Link>
              </li>
            ))}
          </ul>
        ) : <p className="py-8 text-center text-sm text-faint">{d ? 'All clear — nothing needs you right now.' : 'Loading…'}</p>)}
      </div>

      {card('Latest sales today', d && d.recent.length ? (
        <ul className="flex flex-col divide-y divide-border text-sm">
          {d.recent.map((r) => (
            <li key={r.id}>
              <button type="button" onClick={() => setOpen(r.id)} className="flex w-full items-center gap-3 py-2 text-left hover:text-brand">
                <b className="tabular-nums">{r.receiptNo}</b>
                <span className="min-w-0 flex-1 truncate text-muted-foreground">{stamp(r.createdAt)} · {r.cashierName}{r.customerName && ` · ${r.customerName}`}</span>
                <b className="tabular-nums">{money(sym, r.total)}</b>
              </button>
            </li>
          ))}
        </ul>
      ) : <EmptyState title={d ? 'No sales yet today' : 'Loading…'}>{d && <Link to="/till" className="font-semibold text-brand">Open the till →</Link>}</EmptyState>,
      <Link to="/admin/sales" search={{ preset: 'today' }} className="text-[13px] font-semibold text-muted-foreground hover:text-foreground">All sales →</Link>)}
      {open && <SaleSheet id={open} onClose={() => setOpen(null)} />}
    </div>
  )
}
