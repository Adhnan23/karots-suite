import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'
import { AlertTriangle } from 'lucide-react'
import { type ReactNode, useMemo } from 'react'
import type { AdminFinance } from '@/api/types.gen'
import { ChartCard, ChartTable as Table } from '@/components/charts/ChartCard'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { RangePicker } from '@/features/counter/RangePicker'
import { ExportMenu } from '@/features/reports/kit/ExportMenu'
import { Kpi } from '@/features/reports/kit/Kpi'
import { rangeQS } from '@/features/counter/range'
import { Seg } from '@/features/till/bits'
import { configQuery } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { financeQuery } from '../queries'
import { categoryOption, methodLabel, paymentOption, trendOption } from './charts'

const route = getRouteApi('/_authed/admin/finance')

/** Finance (karots-pos /admin/finance): how the business did over a range —
 * Overview, the profit statement, and the money owed either way. */
export function FinancePage() {
  const s = route.useSearch()
  const r = { preset: s.preset ?? 'this-month', from: s.from ?? '', to: s.to ?? '' }
  const tab = s.tab ?? 'overview'
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof s>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery(financeQuery(r)).data
  return (
    <>
      <PageHead title="Finance" sub="How the business did — sales, profit, and the money owed either way."
        actions={<ExportMenu url={`/api/v1/admin/finance?${(rangeQS(r) || '&preset=this-month').slice(1)}`} />} />
      <div className="mb-4 grid gap-2 md:flex md:flex-wrap md:items-center md:justify-between">
        <Seg label="Finance view" value={tab} onChange={(t) => set({ tab: t })} options={[['overview', 'Overview'], ['profit', 'Profit'], ['cash', 'Money owed']]} />
        <span className="flex flex-wrap items-center gap-2">
          <RangePicker full value={r} onChange={(x) => set({ preset: x.preset || 'this-month', from: x.from, to: x.to })} />
          {d && <small className="text-[13px] text-muted-foreground">{d.from === d.to ? d.from : `${d.from} – ${d.to}`}</small>}
        </span>
      </div>
      {!d ? <EmptyState title="Loading…" /> : tab === 'profit' ? <ProfitTab d={d} sym={sym} /> : tab === 'cash' ? <CashTab d={d} sym={sym} /> : <OverviewTab d={d} sym={sym} />}
    </>
  )
}

function OverviewTab({ d, sym }: { d: AdminFinance; sym: string }) {
  const trend = useMemo(() => trendOption(d.trend, sym, d.grain), [d, sym])
  const pay = useMemo(() => paymentOption(d.byPayment, sym), [d, sym])
  const grainWord = { day: 'day', week: 'week', month: 'month' }[d.grain] ?? d.grain
  return (
    <div className="flex flex-col gap-4">
      <section aria-label="Headline" className={cn(SURFACE, 'grid grid-cols-2 divide-border md:grid-cols-5 md:divide-x [&>*]:border-border phone:[&>*]:border-b')}>
        <Kpi amount={d.revenue} label="Revenue" v={money(sym, d.revenue)} sub={`${d.salesCount} sale${d.salesCount === 1 ? '' : 's'}`} />
        <Kpi amount={d.grossProfit} label="Gross profit" v={money(sym, d.grossProfit)} sub={`${d.grossMargin}% margin`} tone="sign" />
        <Kpi amount={d.netProfit} label="Net profit" v={money(sym, d.netProfit)} sub="after expenses and losses" tone="sign" />
        <Kpi amount={d.expenses} label="↑ Expenses" v={money(sym, d.expenses)} tone="out" />
        <Kpi amount={d.received} label="↓ Received at sale" v={money(sym, d.received)} tone="in" className="phone:col-span-2" />
      </section>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        <ChartCard title="Revenue and profit" sub={`By ${grainWord}`} option={trend} label={`Revenue and profit by ${grainWord}`} empty={!d.trend.length}
          table={<Table head={['Period', 'Revenue', 'Profit']} rows={d.trend.map((p) => [p.day, money(sym, p.revenue), money(sym, p.profit)])} />} />
        <ChartCard title="How it was paid" option={pay} label="Takings by payment method" empty={!d.byPayment.length} height={240}
          table={<Table head={['Method', 'Sales', 'Total']} rows={d.byPayment.map((p) => [methodLabel(p.method), p.count, money(sym, p.total)])} />} />
      </div>
    </div>
  )
}

function ProfitTab({ d, sym }: { d: AdminFinance; sym: string }) {
  const cats = useMemo(() => categoryOption(d.categories, sym), [d, sym])
  const line = (label: string, v: string, opts: { sign?: '+' | '−'; strong?: boolean; skipZero?: boolean } = {}) =>
    opts.skipZero && !num(v) ? null : (
      <li key={label} className={cn('flex justify-between gap-3 py-1.5', opts.strong && 'border-t-2 border-foreground pt-2 font-bold')}>
        <span className={cn(!opts.strong && 'text-muted-foreground')}>{opts.sign && `${opts.sign} `}{label}</span>
        <span className="tabular-nums">{money(sym, v)}</span>
      </li>
    )
  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)]">
      <section aria-label="Profit and loss" className={cn(SURFACE, 'p-5')}>
        <h2 className="mb-2 font-display text-lg font-bold">Profit and loss</h2>
        <ul className="divide-y divide-border text-sm">
          {line(num(d.tax) > 0 ? 'Sales (before tax)' : 'Sales', d.grossRevenue)}
          {line('Returns', d.returns, { sign: '−', skipZero: true })}
          {line('Net revenue', d.revenue, { strong: true })}
          {line('Cost of goods sold', d.cogs, { sign: '−' })}
          {line('Gross profit', d.grossProfit, { strong: true })}
          {line('Expenses', d.expenses, { sign: '−' })}
          {line('Losses (damage, warranty)', d.losses, { sign: '−', skipZero: true })}
          {line('Own use', d.ownUse, { sign: '−', skipZero: true })}
          {line('Staff welfare', d.staffWelfare, { sign: '−', skipZero: true })}
          {line('Stock corrections', d.stockCorrections, { sign: '−', skipZero: true })}
          {line('Recovered from suppliers', d.recoveries, { sign: '+', skipZero: true })}
          {line('Other income (interest)', d.otherIncome, { sign: '+', skipZero: true })}
          {d.pluginIncome.map((p) => line(p.label, p.amount, { sign: '+' }))}
          {line('Net profit', d.netProfit, { strong: true })}
        </ul>
        {d.zeroCostLines > 0 && (
          <p className="mt-3 flex gap-2 rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] px-3 py-2 text-[13px] text-warning">
            <AlertTriangle size={16} className="mt-0.5 flex-none" />
            <span>{d.zeroCostLines} sale line{d.zeroCostLines === 1 ? '' : 's'} ({money(sym, d.zeroCostRevenue)}) had no cost recorded, so profit reads higher than it was. Set those products' cost.</span>
          </p>
        )}
      </section>
      <div className="flex min-w-0 flex-col gap-4">
        <ChartCard title="Profit by category" sub="Top 10" option={cats} label="Profit by category" empty={!d.categories.length} height={Math.max(160, Math.min(10, d.categories.length) * 34)}
          table={<Table head={['Category', 'Revenue', 'Profit']} rows={d.categories.map((c) => [c.category, money(sym, c.revenue), money(sym, c.profit)])} />} />
        <section aria-label="Top products" className={cn(SURFACE, 'p-4')}>
          <h2 className="mb-2 font-display text-base font-bold">Top products</h2>
          {d.topProducts.length ? <Table head={['Product', 'Qty', 'Revenue', 'Profit']} rows={d.topProducts.map((p) => [p.name, qtyText(p.qty), money(sym, p.revenue), money(sym, p.profit)])} />
            : <p className="py-6 text-center text-sm text-faint">Nothing sold in this range.</p>}
        </section>
      </div>
    </div>
  )
}

function CashTab({ d, sym }: { d: AdminFinance; sym: string }) {
  const card = (title: string, children: ReactNode) => <section aria-label={title} className={cn(SURFACE, 'flex flex-col p-5')}><h2 className="mb-2 font-display text-lg font-bold">{title}</h2>{children}</section>
  const row = (label: string, v: string, link?: ReactNode, tone?: string) => (
    <li className="flex items-baseline justify-between gap-3 py-2">
      <span className="flex flex-col"><span className="text-muted-foreground">{label}</span>{link}</span>
      <b className={cn('font-display text-lg tabular-nums', tone)}>{money(sym, v)}</b>
    </li>
  )
  const go = 'text-[12px] font-semibold text-muted-foreground underline-offset-2 hover:text-foreground hover:underline'
  const diff = num(d.registerDiff)
  return (
    <div className="grid gap-4 md:grid-cols-2">
      {card('In this range', (
        <ul className="divide-y divide-border text-sm">
          {row('↓ Received at sale', d.received, undefined, 'text-success')}
          {num(d.tax) > 0 && row('Tax collected — owed to the tax office', d.tax)}
          {row('↑ Taken out of the tills', d.cashWithdrawn)}
          {row(diff < 0 ? 'Tills came up short' : diff > 0 ? 'Tills came up over' : 'Tills balanced', String(Math.abs(diff)), <Link to="/admin/tills" className={go}>Till sessions</Link>, diff < 0 ? 'text-danger' : diff > 0 ? 'text-success' : undefined)}
          <li className="py-2"><Link to="/admin/cashflow" className={go}>Every movement → Cash flow</Link></li>
        </ul>
      ))}
      {card('Owed right now', (
        <ul className="divide-y divide-border text-sm">
          {row('Customers owe you', d.receivables, <Link to="/admin/customers" className={go}>Customers</Link>)}
          {row('Customers paid ahead', d.customerAdvances)}
          {row('You owe suppliers', d.payables, <Link to="/admin/suppliers" search={{ show: 'owing' }} className={go}>Suppliers you owe</Link>, num(d.payables) > 0 ? 'text-danger' : undefined)}
          {row('Suppliers hold for you', d.supplierCredit, <Link to="/admin/suppliers" search={{ show: 'credit' }} className={go}>Suppliers in credit</Link>)}
        </ul>
      ))}
    </div>
  )
}
