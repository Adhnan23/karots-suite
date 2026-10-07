import { keepPreviousData, useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { AlertTriangle, ChevronRight } from 'lucide-react'
import { DataList } from '@/components/admin/DataList'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { countText } from '@/features/counter/queries'
import { configQuery } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { BarCell } from '../kit/BarCell'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { ReportPage } from '../kit/ReportPage'
import { reportQS } from '../kit/search'
import { inventoryProductsQuery, inventoryQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/inventory')

/** Inventory valuation (karots-pos /admin/reports/inventory): what the stock on
 * hand is worth at cost (by the lots it was bought in) and at retail, broken
 * down by category — tap one to go inside it. */
export function InventoryPage() {
  const s = route.useSearch()
  const f = { category: s.category ?? '', zero: s.zero ?? '' }
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof s>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...inventoryQuery(f), placeholderData: keepPreviousData }).data
  const list = useInfiniteQuery({ ...inventoryProductsQuery(f), placeholderData: keepPreviousData })
  const rows = list.data?.pages.flat() ?? []
  const top = Math.max(0, ...(d?.children.map((c) => num(c.costValue)) ?? []))
  const crumb = 'rounded-full px-2.5 py-1 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground'
  return (
    <ReportPage title="Inventory valuation" sub="What the stock on hand is worth — at what you paid for it, and at what it sells for."
      exportUrl={`/api/v1/admin/reports/inventory?${reportQS({ preset: '', ...f })}`}
      filters={(
        <label className="flex items-center gap-2 text-sm font-semibold text-muted-foreground">
          <input type="checkbox" checked={f.zero === '1'} onChange={(e) => set({ zero: e.target.checked ? '1' : '' })} className="h-4 w-4 accent-[var(--brand)]" />
          Show products with none on hand
        </label>
      )}>
      <nav aria-label="Category path" className="-mb-1 flex flex-wrap items-center gap-0.5">
        <button type="button" className={cn(crumb, !f.category && 'text-foreground')} onClick={() => set({ category: '' })}>Whole shop</button>
        {d?.breadcrumb.map((c) => (
          <span key={c.id} className="flex items-center gap-0.5"><ChevronRight size={14} className="text-faint" />
            <button type="button" className={cn(crumb, String(c.id) === f.category && 'text-foreground')} onClick={() => set({ category: String(c.id) })}>{c.name}</button>
          </span>
        ))}
      </nav>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label="At cost" v={d ? money(sym, d.costValue) : '…'} sub="what you paid" />
        <Kpi label="At selling price" v={d ? money(sym, d.retailValue) : '…'} />
        <Kpi label="Profit waiting on the shelf" amount={d?.margin} tone="sign" v={d ? money(sym, d.margin) : '…'} sub={d && `${d.marginPct}% of retail`} />
        <Kpi label="Products in stock" v={d ? `${d.inStock} of ${d.skus}` : '…'} sub={d?.units.slice(0, 3).map((u) => `${qtyText(u.qty)} ${u.unit}`).join(' · ')} />
      </KpiStrip>
      {d && (d.missingCost > 0 || d.negativeQty > 0) && (
        <p className="flex gap-2 rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] px-3 py-2 text-[13px] text-warning">
          <AlertTriangle size={16} className="mt-0.5 flex-none" />
          <span>
            {d.missingCost > 0 && `${d.missingCost} product${d.missingCost === 1 ? '' : 's'} in stock have no cost, so the cost value reads low. `}
            {d.negativeQty > 0 && `${d.negativeQty} product${d.negativeQty === 1 ? '' : 's'} show less than none on hand — count them.`}
          </span>
        </p>
      )}
      {d && d.children.length > 0 && (
        <section aria-label="By category" className={cn(SURFACE, 'p-4')}>
          <h2 className="mb-2 font-display text-base font-bold">By category</h2>
          <ul className="flex flex-col divide-y divide-border text-sm">
            {d.children.map((c) => (
              <li key={`${c.id}-${c.direct}`}>
                <button type="button" disabled={c.direct} onClick={() => set({ category: String(c.id) })}
                  className="grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 py-2 text-left enabled:hover:text-brand md:grid-cols-[minmax(0,1fr)_120px_minmax(140px,30%)_auto]">
                  <span className="flex min-w-0 flex-col"><b className="truncate">{c.direct ? `Filed on ${c.name} itself` : c.name}</b><small className="text-faint">{c.inStock} of {c.skus} in stock</small></span>
                  <span className="hidden text-right tabular-nums text-muted-foreground md:block">{money(sym, c.retailValue)}</span>
                  <span className="hidden md:block"><BarCell pct={top > 0 ? (num(c.costValue) / top) * 100 : null}>{money(sym, c.costValue)}</BarCell></span>
                  <span className="flex items-center gap-1 md:hidden"><b className="tabular-nums">{money(sym, c.costValue)}</b></span>
                  {!c.direct && c.hasChildren ? <ChevronRight size={16} className="hidden text-faint md:block" /> : <span className="hidden w-4 md:block" />}
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
      <div>
        <div className="mb-2 text-right text-[13px] text-muted-foreground">{countText(rows.length, !!list.hasNextPage)} products, most valuable first</div>
        <DataList label="Products" rows={rows} rowKey={(x) => x.id}
          empty={<EmptyState title={list.isPending ? 'Loading…' : 'No stock here'} />}
          footer={list.hasNextPage && (
            <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
              className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60 print:hidden">
              {list.isFetchingNextPage ? 'Loading…' : 'Show more'}
            </button>
          )}
          columns={[
            { key: 'name', header: 'Product', cell: (x) => <span className="flex flex-col"><b>{x.name}</b><small className="font-normal text-faint">{x.category}{x.barcode && ` · ${x.barcode}`}</small></span> },
            { key: 'qty', header: 'On hand', num: true, cell: (x) => <span className={cn(num(x.qty) < 0 && 'text-danger')}>{qtyText(x.qty)} {x.unit}</span> },
            { key: 'cost', header: 'At cost', num: true, cell: (x) => <b>{money(sym, x.costValue)}</b> },
            { key: 'retail', header: 'At selling price', num: true, wide: true, cell: (x) => money(sym, x.retailValue) },
          ]} />
      </div>
    </ReportPage>
  )
}
