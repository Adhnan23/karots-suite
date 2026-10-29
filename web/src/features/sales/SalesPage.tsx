import { keepPreviousData, useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Download, Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Tag } from '@/features/admin/products/ProductsPage'
import { countText } from '@/features/counter/queries'
import { RangePicker } from '@/features/counter/RangePicker'
import { stamp } from '@/features/money/format'
import { adminSalesQuery, adminSalesSummaryQuery, salesQS } from '@/features/money/queries'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { METHODS, STATUS, STATUS_TAG } from './filters'
import { SaleSheet } from './SaleSheet'

const route = getRouteApi('/_authed/admin/sales')

/** Sales: every sale by range, status, payment and
 * search — the totals over every match, export, and a sheet per sale with
 * reprint and return. */
export function SalesPage() {
  const s = route.useSearch()
  const f = { preset: s.preset ?? 'this-month', from: s.from ?? '', to: s.to ?? '', status: s.status ?? '', method: s.method ?? '', q: s.q ?? '' }
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof f>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q.trim(), 250)
  useEffect(() => { if (dq !== f.q) set({ q: dq }) }, [dq]) // eslint-disable-line react-hooks/exhaustive-deps
  const sum = useQuery({ ...adminSalesSummaryQuery(f), placeholderData: keepPreviousData }).data
  const list = useInfiniteQuery({ ...adminSalesQuery(f), placeholderData: keepPreviousData })
  const rows = list.data?.pages.flat() ?? []
  const [open, setOpen] = useState<number | null>(null)
  const sel = cn(INPUT, 'appearance-auto md:w-44')
  const stat = (label: string, v: string, cls?: string) => (
    <div className="flex flex-col gap-0.5 px-4 py-3"><small className="text-[12px] font-semibold text-muted-foreground">{label}</small><b className={cn('font-display text-xl tabular-nums', cls)}>{v}</b></div>
  )

  return (
    <>
      <PageHead title="Sales" sub="Every sale — find one, print it again, or take goods back."
        actions={(
          <DropdownMenu>
            <DropdownMenuTrigger asChild><Button variant="outline" className="h-10 gap-1.5 px-4"><Download size={16} /> Export</Button></DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {(['csv', 'xlsx', 'ods'] as const).map((fmt) => (
                <DropdownMenuItem key={fmt} asChild><a href={`/api/v1/admin/sales/export?format=${fmt}&${salesQS(f)}`}>{fmt.toUpperCase()}</a></DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        )} />
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <div className="relative md:w-72">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search sales" placeholder="Receipt, customer or product" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        <RangePicker full value={f} onChange={(r) => set(r)} />
        <div className="grid grid-cols-2 gap-2 md:flex">
          <select aria-label="Status" value={f.status} onChange={(e) => set({ status: e.target.value })} className={sel}>{STATUS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</select>
          <select aria-label="Payment" value={f.method} onChange={(e) => set({ method: e.target.value })} className={sel}>{METHODS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</select>
        </div>
      </div>
      <section aria-label="Totals" className={cn(SURFACE, 'mb-3 grid grid-cols-2 divide-border md:grid-cols-4 md:divide-x')}>
        {stat('Sales', sum ? String(sum.count) : '…')}
        {stat('Total', sum ? money(sym, sum.total) : '…')}
        {stat('↓ Paid', sum ? money(sym, sum.paid) : '…', 'text-success')}
        {stat(`Profit${sum ? ` · ${sum.margin}%` : ''}`, sum ? money(sym, sum.profit) : '…', sum && num(sum.profit) < 0 ? 'text-danger' : undefined)}
      </section>
      <div className="mb-2 text-right text-[13px] text-muted-foreground">{countText(rows.length, !!list.hasNextPage)} shown</div>
      <DataList label="Sales" rows={rows} rowKey={(r) => r.id} onRow={(r) => setOpen(r.id)}
        empty={<EmptyState title={list.isPending ? 'Loading…' : 'No sales match'} />}
        footer={list.hasNextPage && (
          <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
            className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60">
            {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
          </button>
        )}
        columns={[
          { key: 'sale', header: 'Sale', cell: (r) => (
            <span className="flex flex-col">
              <span className="flex items-center gap-1.5"><b className="tabular-nums">{r.receiptNo}</b>{STATUS_TAG[r.status] && <Tag tone={STATUS_TAG[r.status][1]}>{STATUS_TAG[r.status][0]}</Tag>}</span>
              <small className="font-normal text-faint">{stamp(r.createdAt)}{r.customerName && ` · ${r.customerName}`}</small>
            </span>
          ) },
          { key: 'cashier', header: 'Cashier', wide: true, cell: (r) => <span className="text-muted-foreground">{r.cashierName}</span> },
          { key: 'total', header: 'Total', num: true, cell: (r) => <b>{money(sym, r.total)}</b> },
          { key: 'paid', header: 'Paid', num: true, wide: true, cell: (r) => money(sym, r.paid) },
          { key: 'profit', header: 'Profit', num: true, cell: (r) => <span className={cn(num(r.profit) < 0 && 'text-danger')}>{money(sym, r.profit)}</span> },
        ]} />
      {open && <SaleSheet id={open} onClose={() => setOpen(null)} />}
    </>
  )
}
