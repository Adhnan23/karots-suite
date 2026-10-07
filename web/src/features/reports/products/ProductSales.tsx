import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useMemo } from 'react'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { ChartCard, ChartTable } from '@/components/charts/ChartCard'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { Seg } from '@/features/till/bits'
import { configQuery } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { productPeriodOption } from '../charts'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { delta, periodText } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf } from '../kit/search'
import { productSalesQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/product-sales')

/** Product sales (karots-pos /admin/reports/product-sales): one product's units
 * and takings over time, beside the same time last year — the feel for how much
 * to reorder. */
export function ProductSalesPage() {
  const s = route.useSearch()
  const r = rangeOf(s)
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof s>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const q = { ...r, product: s.product ? String(s.product) : '', grain: s.grain ?? '' }
  const d = useQuery({ ...productSalesQuery(q), placeholderData: keepPreviousData }).data
  const option = useMemo(() => d && productPeriodOption(d.rows, d.grain), [d])
  const picked = !!s.product && d?.productId === Number(s.product)

  return (
    <ReportPage title="Product sales" sub="One product over time, beside the same time last year."
      range={r} onRange={(x) => set(x)} caption={d && periodText(d.from, d.to)}
      filters={(
        <>
          <ProductPicker title="Product to show" onPick={(p) => set({ product: p.id })} trigger={(open) => (
            <button type="button" aria-label="Product" onClick={open} className={cn(INPUT, 'truncate text-left md:w-72', !picked && 'text-faint')}>{picked ? d.name : 'Pick a product'}</button>
          )} />
          <Seg label="Group by" value={d?.grain ?? 'day'} onChange={(g) => set({ grain: g })} options={[['day', 'Day'], ['week', 'Week'], ['month', 'Month']]} />
        </>
      )}>
      {!picked ? (
        <div className={SURFACE}><EmptyState title={s.product ? 'Loading…' : 'Pick a product'}>Its units and takings in this period show here, with last year's beside them.</EmptyState></div>
      ) : (
        <>
          <KpiStrip label="Headline" cols={3}>
            <Kpi label="Units sold" v={qtyText(d.qty)} change={delta(d.qty, d.lastYearQty)} sub={`${qtyText(d.lastYearQty)} this time last year`} />
            <Kpi label="Revenue" amount={d.revenue} v={money(sym, d.revenue)} />
            <Kpi label="Per period" v={qtyText(String(d.rows.length ? Math.round((Number(d.qty) / d.rows.length) * 10) / 10 : 0))} sub={`units a ${d.grain}`} className="max-md:col-span-2" />
          </KpiStrip>
          {option && (
            <ChartCard title="Units sold" sub={`By ${d.grain}`} option={option} label={`${d.name} units by ${d.grain}, this period and last year`} empty={!d.rows.length}
              table={<ChartTable head={['Period', 'Units', 'Last year', 'Revenue']} rows={d.rows.map((x) => [x.day, qtyText(x.qty), qtyText(x.lastYearQty), money(sym, x.revenue)])} />} />
          )}
        </>
      )}
    </ReportPage>
  )
}
