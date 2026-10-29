import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router'
import { PackageCheck, Plus, Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { PurchaseRow } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { suppliersQuery } from '@/features/counter/queries'
import { Seg } from '@/features/till/bits'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { purchasesQuery } from '../queries'

const route = getRouteApi('/_authed/admin/purchases/')
export const day = (iso: string) => new Date(iso).toLocaleDateString([], { day: 'numeric', month: 'short', year: '2-digit' })

/** Where a purchase stands, in words. */
export function StatusTag({ p }: { p: PurchaseRow }) {
  if (p.status === 'draft') return p.expectedDate && p.expectedDate < new Date().toISOString().slice(0, 10) ? <Tag tone="danger">late</Tag> : <Tag tone="brand">ordered</Tag>
  const bal = num(p.balance)
  if (bal <= 0) return <Tag tone="muted">settled</Tag>
  return p.dueDate && p.dueDate < new Date().toISOString().slice(0, 10) ? <Tag tone="danger">overdue</Tag> : <Tag tone="warn">{num(p.paid) + num(p.credited) > 0 ? 'part paid' : 'unpaid'}</Tag>
}

/** Purchases: orders waiting to come in, and
 * every delivery received (GRN) with what's still owed on it. */
export function PurchasesPage() {
  const s = route.useSearch()
  const f = { tab: s.tab ?? 'orders', q: s.q ?? '', supplier: s.supplier ?? 0, unpaid: !!s.unpaid, page: s.page ?? 1 }
  const navigate = route.useNavigate()
  const go = useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const sups = useQuery(suppliersQuery('')).data ?? []
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q, 250)
  useEffect(() => { if (dq !== f.q) void navigate({ search: (x) => ({ ...x, q: dq, page: 1 }), replace: true }) }, [dq, f.q, navigate])
  const data = useQuery({ ...purchasesQuery(f), placeholderData: keepPreviousData }).data
  const set = (patch: Partial<typeof f>) => void navigate({ search: (x) => ({ ...x, page: 1, ...patch }) })
  const orders = f.tab === 'orders'
  const chip = 'rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

  return (
    <>
      <PageHead title="Purchases" sub={orders ? 'Orders placed with suppliers, waiting to come in.' : 'Deliveries received (GRN) and what is still owed on each.'}
        actions={<>
          <Button variant="outline" className="h-10 gap-1.5 px-4" asChild><Link to="/admin/purchases/new" search={{ receive: true }}><PackageCheck size={16} /> Receive delivery</Link></Button>
          <Button className="h-10 gap-1.5 px-4 font-bold" asChild><Link to="/admin/purchases/new"><Plus size={16} /> New order</Link></Button>
        </>} />
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <Seg label="Show" value={f.tab} onChange={(tab) => set({ tab, unpaid: false })} options={[['orders', 'Orders'], ['received', 'Received']]} />
        <div className="relative md:w-72">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search purchases" placeholder="Invoice no or supplier" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        <select aria-label="Supplier" value={f.supplier} onChange={(e) => set({ supplier: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto md:w-60')}>
          <option value={0}>All suppliers</option>
          {sups.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
        </select>
        {!orders && <button type="button" aria-pressed={f.unpaid} onClick={() => set({ unpaid: !f.unpaid })} className={chip}>Still owed</button>}
      </div>
      <DataList label="Purchases" rows={data?.items ?? []} rowKey={(r) => r.id} onRow={(r) => void go({ to: '/admin/purchases/$id', params: { id: String(r.id) } })}
        empty={<EmptyState title={!data ? 'Loading…' : f.q || f.supplier || f.unpaid ? 'Nothing matches' : orders ? 'No open orders' : 'Nothing received yet'}>
          {data && !f.q && !f.supplier && (orders ? 'Place an order with a supplier, or build them from Reorder.' : 'Receive a delivery to add stock and what you owe for it.')}
        </EmptyState>}
        footer={data && <Pager page={data.page} pageSize={data.pageSize} total={data.total} onPage={(page) => void navigate({ search: (x) => ({ ...x, page }) })} />}
        columns={[
          { key: 'who', header: orders ? 'Order' : 'Delivery', cell: (r) => (
            <span className="flex flex-col gap-0.5">
              <span className="flex flex-wrap items-center gap-1.5"><b>{r.supplierName}</b><StatusTag p={r} /></span>
              <small className="font-normal text-faint">{orders ? `#${r.id}` : r.invoiceNo || `#${r.id}`} · {day(r.date)}{r.by && ` · ${r.by}`}</small>
            </span>
          ) },
          orders
            ? { key: 'exp', header: 'Expected', cell: (r) => <span className="text-muted-foreground">{r.expectedDate ? day(r.expectedDate) : '—'}</span> }
            : { key: 'due', header: 'Due', wide: true, cell: (r) => <span className="text-muted-foreground">{r.dueDate ? day(r.dueDate) : '—'}</span> },
          { key: 'total', header: 'Total', num: true, cell: (r) => <b>{money(sym, r.total)}</b> },
          ...(orders ? [] : [{ key: 'bal', header: 'Owed', num: true, cell: (r: PurchaseRow) => <span className={cn(num(r.balance) > 0 ? 'text-danger' : 'text-faint')}>{num(r.balance) > 0 ? money(sym, r.balance) : '—'}</span> }]),
        ]} />
    </>
  )
}
