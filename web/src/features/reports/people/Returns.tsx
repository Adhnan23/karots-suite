import { keepPreviousData, useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useState } from 'react'
import { DataList } from '@/components/admin/DataList'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { countText } from '@/features/counter/queries'
import { stamp } from '@/features/money/format'
import { SaleSheet } from '@/features/sales/SaleSheet'
import { configQuery } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { BarCell } from '../kit/BarCell'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { periodText } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { returnLinesQuery, returnsQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/returns')

/** Returns (karots-pos /admin/reports/returns): what went back and what was
 * refunded, by the day it went back — and which products come back most. */
export function ReturnsPage() {
  const r = rangeOf(route.useSearch())
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...returnsQuery(r), placeholderData: keepPreviousData }).data
  const list = useInfiniteQuery({ ...returnLinesQuery(r), placeholderData: keepPreviousData })
  const rows = list.data?.pages.flat() ?? []
  const [open, setOpen] = useState<number | null>(null)
  const top = num(d?.top[0]?.refund ?? '0')
  return (
    <ReportPage title="Returns" sub="Goods taken back and money refunded, by the day it went back."
      range={r} onRange={(x) => void navigate({ search: (p) => ({ ...p, ...x }), replace: true })} caption={d && periodText(d.from, d.to)}
      exportUrl={`/api/v1/admin/reports/returns?${reportQS(r)}`}>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label="↑ Refunded" amount={d?.refund} tone="out" v={d ? money(sym, d.refund) : '…'} />
        <Kpi label="Units back" v={d ? qtyText(d.qty) : '…'} />
        <Kpi label="Returns" v={d ? String(d.returns) : '…'} />
        <Kpi label="Receipts" v={d ? String(d.receipts) : '…'} sub="sales something came back from" />
      </KpiStrip>
      {d && d.top.length > 0 && (
        <section aria-label="Most returned" className={cn(SURFACE, 'p-4')}>
          <h2 className="mb-2 font-display text-base font-bold">Most returned</h2>
          <ul className="flex flex-col gap-1.5 text-sm">
            {d.top.map((p) => (
              <li key={p.name} className="grid grid-cols-[minmax(0,1fr)_auto_minmax(110px,40%)] items-center gap-3">
                <span className="truncate">{p.name}</span><small className="tabular-nums text-faint">{qtyText(p.qty)} back</small>
                <BarCell pct={top > 0 ? (num(p.refund) / top) * 100 : null} tone="danger">{money(sym, p.refund)}</BarCell>
              </li>
            ))}
          </ul>
        </section>
      )}
      <div>
        <div className="mb-2 flex flex-wrap justify-between gap-2 text-[13px] text-muted-foreground">
          <span>The P&amp;L takes a return off in its sale’s own period.</span><span>{countText(rows.length, !!list.hasNextPage)} shown</span>
        </div>
        <DataList label="Returned lines" rows={rows} rowKey={(x) => `${x.when}-${x.receiptNo}-${x.product}-${x.qty}`} onRow={(x) => setOpen(x.saleId)}
          empty={<EmptyState title={list.isPending ? 'Loading…' : 'Nothing came back in this period'} />}
          footer={list.hasNextPage && (
            <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
              className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60 print:hidden">
              {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
            </button>
          )}
          columns={[
            { key: 'product', header: 'Product', cell: (x) => (
              <span className="flex flex-col"><b>{x.product}</b><small className="font-normal text-faint">{stamp(x.when)} · {x.receiptNo}, sold {x.saleDate}</small></span>
            ) },
            { key: 'qty', header: 'Qty', num: true, cell: (x) => qtyText(x.qty) },
            { key: 'refund', header: 'Refund', num: true, cell: (x) => <b>{money(sym, x.refund)}</b> },
            { key: 'who', header: 'Customer', wide: true, cell: (x) => <span className="text-muted-foreground">{x.customer || '—'}</span> },
            { key: 'why', header: 'Reason', wide: true, cell: (x) => <span className="text-muted-foreground">{x.reason || '—'}</span> },
          ]} />
      </div>
      {open && <SaleSheet id={open} onClose={() => setOpen(null)} />}
    </ReportPage>
  )
}
