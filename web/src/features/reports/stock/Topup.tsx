import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'
import { DataList } from '@/components/admin/DataList'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { ChartTable } from '@/components/charts/ChartCard'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { configQuery } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { ReportPage } from '../kit/ReportPage'
import { topupQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/topup')

/** Top-up & reorder (karots-pos /admin/reports/topup-reorder): what needs
 * topping up — stock under its reorder level, and any plugin floats — against
 * the money you have to pay for it. A live view, no period. */
export function TopupPage() {
  const s = route.useSearch()
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...topupQuery({ category: s.category ?? '' }), placeholderData: keepPreviousData }).data
  const gap = num(d?.gap ?? '0')
  const places = (title: string, rows: { name: string; balance: string }[], total: string) => (
    <section aria-label={title} className={cn(SURFACE, 'p-4')}>
      <h2 className="mb-2 font-display text-base font-bold">{title}</h2>
      <ChartTable head={['Place', 'Balance']} rows={[...rows.map((p) => [p.name, money(sym, p.balance)]), [<b key="t">Total</b>, <b key="v">{money(sym, total)}</b>]]} />
    </section>
  )
  return (
    <ReportPage title="Top-up and reorder" sub="What needs buying or topping up, and whether the money on hand covers it."
      filters={(
        <>
          <div className="md:w-72"><CategoryPicker label="Category" noneLabel="Every category" value={Number(s.category) || 0} onChange={(id) => void navigate({ search: { category: id ? String(id) : undefined }, replace: true })} /></div>
          <Button variant="outline" className="h-10 px-4 md:ml-auto" asChild><Link to="/admin/reorder">Order them →</Link></Button>
        </>
      )}>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label="To restock" v={d ? money(sym, d.stockNeed) : '…'} sub={d && `${d.stock.length} product${d.stock.length === 1 ? '' : 's'} at cost`} />
        <Kpi label="To top up" v={d ? money(sym, d.pluginNeed) : '…'} sub="floats and the like" />
        <Kpi label="Money on hand" v={d ? money(sym, d.available) : '…'} sub="lockers, bank and open tills" />
        <Kpi label={gap < 0 ? 'Short by' : 'Left over'} v={d ? money(sym, String(Math.abs(gap))) : '…'} tone={gap < 0 ? 'out' : 'in'} amount={String(Math.abs(gap))} />
      </KpiStrip>
      {d?.plugins.map((g) => (
        <section key={g.title} aria-label={g.title} className={cn(SURFACE, 'p-4')}>
          <h2 className="mb-2 flex justify-between font-display text-base font-bold"><span>{g.title}</span><span className="tabular-nums">{money(sym, g.need)}</span></h2>
          <ChartTable head={['Item', 'Have', 'Target', 'Need']} rows={g.rows.map((r) => [r.name, r.have, r.target, money(sym, r.need)])} />
        </section>
      ))}
      <DataList label="Stock to restock" rows={d?.stock ?? []} rowKey={(x) => x.productId}
        empty={<EmptyState title={d ? 'Nothing is under its reorder level' : 'Loading…'} />}
        columns={[
          { key: 'p', header: 'Product', cell: (x) => <span className="flex flex-col"><b>{x.product}</b><small className="font-normal text-faint">{x.category}</small></span> },
          { key: 'on', header: 'On hand', num: true, cell: (x) => <span className={cn(num(x.onHand) <= 0 && 'font-bold text-danger')}>{qtyText(x.onHand)}</span> },
          { key: 'lvl', header: 'Reorder at', num: true, wide: true, cell: (x) => x.reorder },
          { key: 'short', header: 'Short', num: true, wide: true, cell: (x) => qtyText(x.short) },
          { key: 'cost', header: 'To restock', num: true, cell: (x) => <b>{money(sym, x.restock)}</b> },
        ]} />
      {d && (
        <div className="grid gap-4 md:grid-cols-2">
          {places('Cash', d.physical, d.physicalTotal)}
          {places('Bank and card', d.bank, d.bankTotal)}
        </div>
      )}
    </ReportPage>
  )
}
