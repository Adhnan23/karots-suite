import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Download, Search, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { Tag } from '@/features/admin/products/ProductsPage'
import { productsQuery } from '@/features/admin/products/queries'
import { rangeQS } from '@/features/counter/range'
import { RangePicker } from '@/features/counter/RangePicker'
import { Seg } from '@/features/till/bits'
import { configQuery } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { AdjustForm } from './AdjustForm'
import { LotsSheet } from './LotsSheet'
import { MOVE_TYPES, moveLabel, movementsQuery, staffQuery } from './queries'

const route = getRouteApi('/_authed/admin/stock/')
const SELECT = cn(INPUT, 'appearance-auto')

/** On-hand levels and the movement history (karots-pos "Stock Movement"). */
export function StockPage() {
  const s = route.useSearch()
  const navigate = route.useNavigate()
  const view = s.view ?? 'levels'
  return (
    <>
      <PageHead title="Stock" sub={view === 'levels' ? 'What’s on the shelf. Tap a product for its lots, or adjust a count.' : 'Every unit in and out — who, when and why.'}
        actions={<Seg label="View" value={view} onChange={(v) => void navigate({ search: { view: v } })} options={[['levels', 'On hand'], ['moves', 'Movements']]} />} />
      {view === 'levels' ? <Levels /> : <Moves />}
    </>
  )
}

function Levels() {
  const s = route.useSearch()
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const f = { q: s.q ?? '', category: s.category ?? 0, low: !!s.low, disabled: false, page: s.page ?? 1 }
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q, 250)
  useEffect(() => { if (dq !== f.q) void navigate({ search: (x) => ({ ...x, q: dq, page: 1 }), replace: true }) }, [dq, f.q, navigate])
  const data = useQuery({ ...productsQuery(f), placeholderData: keepPreviousData }).data
  const [lots, setLots] = useState(0)
  const [adjust, setAdjust] = useState(0)
  const set = (patch: Partial<typeof f>) => void navigate({ search: (x) => ({ ...x, page: 1, ...patch }) })
  return (
    <>
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <div className="relative md:w-80">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search products" placeholder="Name or barcode" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        <div className="md:w-64"><CategoryPicker label="Filter by category" noneLabel="All categories" value={f.category} onChange={(category) => set({ category })} /></div>
        <button type="button" aria-pressed={f.low} onClick={() => set({ low: !f.low })}
          className="justify-self-start rounded-full border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">Running low</button>
      </div>
      <DataList label="Stock levels" rows={data?.items ?? []} rowKey={(p) => p.id} onRow={(p) => setLots(p.id)}
        empty={<EmptyState title={data ? 'Nothing matches' : 'Loading…'} />}
        footer={data && <Pager page={data.page} pageSize={data.pageSize} total={data.total} onPage={(page) => void navigate({ search: (x) => ({ ...x, page }) })} />}
        columns={[
          { key: 'name', header: 'Product', cell: (p) => <span className="flex flex-col"><span className="flex items-center gap-1.5"><b>{p.name}</b>{p.low && <Tag tone="danger">low</Tag>}</span><small className="text-faint">{p.categoryName}</small></span> },
          { key: 'qty', header: 'On hand', num: true, cell: (p) => <b className={cn(p.low && 'text-warning')}>{qtyText(p.stockQty)} <small className="font-normal text-faint">{p.unitAbbr}</small></b> },
          { key: 'reorder', header: 'Reorder at', num: true, cell: (p) => <span className="text-muted-foreground">{p.reorderLevel || '—'}</span> },
          { key: 'value', header: 'Value at cost', num: true, cell: (p) => <span className="text-muted-foreground">{money(sym, Math.max(0, num(p.stockQty)) * num(p.costPrice))}</span> },
          { key: 'adj', header: '', wide: true, interactive: true, className: 'w-[1%]', cell: (p) => <Button variant="ghost" className="h-9 px-3" onClick={() => setAdjust(p.id)}>Adjust</Button> },
        ]} />
      {lots > 0 && <LotsSheet key={lots} productId={lots} onClose={() => setLots(0)} onAdjust={() => { setAdjust(lots); setLots(0) }} />}
      {adjust > 0 && <AdjustForm key={adjust} productId={adjust} onClose={() => setAdjust(0)} />}
    </>
  )
}

function Moves() {
  const s = route.useSearch()
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const staff = useQuery(staffQuery).data ?? []
  const f = { product: s.product ?? 0, type: s.type ?? '', user: s.user ?? 0, range: { preset: s.preset ?? '', from: s.from ?? '', to: s.to ?? '' }, page: s.page ?? 1 }
  const data = useQuery({ ...movementsQuery(f), placeholderData: keepPreviousData }).data
  const [productName, setProductName] = useState('')
  const set = (patch: Record<string, unknown>) => void navigate({ search: (x) => ({ ...x, page: 1, ...patch }) })
  return (
    <>
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <ProductPicker title="Filter by product" onPick={(p) => { setProductName(p.name); set({ product: p.id }) }} trigger={(open) => (
          <span className="flex md:w-64">
            <button type="button" onClick={open} className={cn(INPUT, 'flex-1 truncate text-left', !f.product && 'text-faint')}>{f.product ? productName || `Product #${f.product}` : 'All products'}</button>
            {f.product > 0 && <button type="button" aria-label="All products" onClick={() => set({ product: 0 })} className="-ml-10 grid w-10 place-items-center text-faint"><X size={16} /></button>}
          </span>
        )} />
        <select aria-label="Type" value={f.type} onChange={(e) => set({ type: e.target.value })} className={cn(SELECT, 'md:w-48')}>
          <option value="">Every kind</option>
          {MOVE_TYPES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
        </select>
        <select aria-label="By" value={f.user} onChange={(e) => set({ user: Number(e.target.value) })} className={cn(SELECT, 'md:w-44')}>
          <option value={0}>Anyone</option>
          {staff.map((u) => <option key={u.id} value={u.id}>{u.name}</option>)}
        </select>
        <RangePicker full value={f.range} onChange={(r) => set({ preset: r.preset, from: r.from, to: r.to })} />
        <span className="flex gap-1.5 md:ml-auto">
          {[['csv', 'CSV'], ['xlsx', 'Excel']].map(([fmt, l]) => (
            <a key={fmt} download href={`/api/v1/admin/stock/movements?format=${fmt}&product=${f.product}&type=${f.type}&user=${f.user}${rangeQS(f.range)}`}
              className="flex h-10 items-center gap-1.5 rounded-lg border border-border bg-card px-3 text-sm font-medium hover:bg-muted"><Download size={15} /> {l}</a>
          ))}
        </span>
      </div>
      <DataList label="Stock movements" rows={data?.items ?? []} rowKey={(m) => m.id}
        empty={<EmptyState title={data ? 'No movements' : 'Loading…'}>{data && 'Nothing moved for this filter.'}</EmptyState>}
        footer={data && <Pager page={data.page} pageSize={data.pageSize} total={data.total} onPage={(page) => void navigate({ search: (x) => ({ ...x, page }) })} />}
        columns={[
          { key: 'product', header: 'Product', cell: (m) => <span className="flex flex-col"><b>{m.productName}</b>{m.note && <small className="font-normal text-faint">{m.note}</small>}</span> },
          { key: 'type', header: 'Kind', cell: (m) => <Tag tone="muted">{moveLabel(m.type)}</Tag> },
          { key: 'qty', header: 'Qty', num: true, cell: (m) => <b className={num(m.quantity) >= 0 ? 'text-success' : 'text-danger'}>{num(m.quantity) >= 0 ? '+' : '−'}{qtyText(Math.abs(num(m.quantity)))}</b> },
          { key: 'cost', header: 'Worth', num: true, cell: (m) => <span className="text-muted-foreground">{num(m.cost) ? money(sym, m.cost) : '—'}</span> },
          { key: 'by', header: 'By', cell: (m) => m.userName },
          { key: 'when', header: 'When', cell: (m) => <span className="whitespace-nowrap text-muted-foreground">{new Date(m.createdAt).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}</span> },
        ]} />
    </>
  )
}
