import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Plus, Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { AdminProductRow } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { SpreadsheetMenu } from '@/components/admin/ImportSheet'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { configQuery } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { BarcodeForm } from './BarcodeForm'
import { ProductForm, type Editing } from './ProductForm'
import { productsQuery } from './queries'

const route = getRouteApi('/_authed/admin/products/')

export function Tag({ tone, children }: { tone: 'warn' | 'danger' | 'muted' | 'brand'; children: string }) {
  const c = { warn: 'text-warning bg-[color-mix(in_srgb,var(--warning)_14%,transparent)]', danger: 'text-danger bg-[color-mix(in_srgb,var(--danger)_14%,transparent)]', muted: 'text-muted-foreground bg-muted', brand: 'bg-brand-soft text-foreground' }[tone]
  return <span className={cn('inline-flex rounded-full px-2 py-0.5 text-[11px] font-bold whitespace-nowrap', c)}>{children}</span>
}

/** The catalog (karots-pos /admin/products): search, category (with its
 * sub-categories), low stock, disabled; click a row to edit. */
export function ProductsPage() {
  const search = route.useSearch()
  const f = { q: search.q ?? '', category: search.category ?? 0, low: !!search.low, disabled: !!search.disabled, page: search.page ?? 1 }
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q, 250)
  useEffect(() => { if (dq !== f.q) void navigate({ search: (s) => ({ ...s, q: dq, page: 1 }), replace: true }) }, [dq, f.q, navigate])
  const data = useQuery({ ...productsQuery(f), placeholderData: keepPreviousData }).data
  const [editing, setEditing] = useState<Editing | null>(null)
  const [barcodeFor, setBarcodeFor] = useState<AdminProductRow | null>(null)
  const set = (patch: Partial<typeof f>) => void navigate({ search: (s) => ({ ...s, page: 1, ...patch }) })
  const chip = 'rounded-full border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

  return (
    <>
      <PageHead title="Products" sub={data && `${data.total} ${f.low ? 'running low' : 'products'}${f.category ? ' in this category' : ''}`}
        actions={<><SpreadsheetMenu entity="products" /><Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setEditing({ id: 0 })}><Plus size={16} /> New product</Button></>} />
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <div className="relative md:w-80">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search products" placeholder="Name or barcode" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        <div className="md:w-64"><CategoryPicker label="Filter by category" noneLabel="All categories" value={f.category} onChange={(category) => set({ category })} /></div>
        <div className="flex gap-1.5">
          <button type="button" aria-pressed={f.low} onClick={() => set({ low: !f.low })} className={chip}>Running low</button>
          <button type="button" aria-pressed={f.disabled} onClick={() => set({ disabled: !f.disabled })} className={chip}>Show disabled</button>
        </div>
      </div>
      <DataList label="Products" rows={data?.items ?? []} rowKey={(p) => p.id} onRow={(p) => setEditing({ id: p.id })}
        empty={<EmptyState title={!data ? 'Loading…' : f.q || f.category || f.low ? 'Nothing matches' : 'No products yet'}>
          {data && !f.q && !f.category && !f.low && 'Add your first product, or use Stock intake to add and count in one go.'}
        </EmptyState>}
        footer={data && <Pager page={data.page} pageSize={data.pageSize} total={data.total} onPage={(page) => void navigate({ search: (s) => ({ ...s, page }) })} />}
        columns={[
          { key: 'name', header: 'Product', cell: (p) => <Name p={p} /> },
          // karots-pos "+ Add barcode": give a barcode-less item one without opening the whole form.
          { key: 'code', header: '', wide: true, interactive: true, className: 'w-[1%] whitespace-nowrap', cell: (p) => !p.barcode && p.isActive && (
            <Button variant="ghost" className="h-8 gap-1 px-2 text-[12px]" onClick={() => setBarcodeFor(p)}><Plus size={13} /> Add barcode</Button>
          ) },
          { key: 'cat', header: 'Category', cell: (p) => <span className="text-muted-foreground">{p.categoryName}</span> },
          { key: 'stock', header: 'On hand', num: true, cell: (p) => p.isService ? <span className="text-faint">service</span> : (
            <span className={cn(p.low && 'font-bold text-warning')}>{qtyText(p.stockQty)} <small className="font-normal text-faint">{p.unitAbbr}</small></span>
          ) },
          { key: 'reorder', header: 'Reorder at', num: true, wide: true, cell: (p) => <span className="text-muted-foreground">{p.reorderLevel ? `${p.reorderLevel} ${p.unitAbbr}` : '—'}</span> },
          { key: 'cost', header: 'Cost', num: true, cell: (p) => <span className="text-muted-foreground">{money(sym, p.costPrice)}</span> },
          { key: 'price', header: 'Price', num: true, cell: (p) => (
            <span className="flex flex-col items-end"><b>{money(sym, p.sellingPrice)}</b>{Number(p.wholesalePrice) > 0 && <small className="text-faint">wholesale {money(sym, p.wholesalePrice)}</small>}</span>
          ) },
        ]} />
      {barcodeFor && <BarcodeForm key={barcodeFor.id} p={barcodeFor} onClose={() => setBarcodeFor(null)} />}
      {editing && <ProductForm key={`${editing.id}-${editing.copyOf ?? ''}`} editing={editing} onEdit={setEditing} onClose={() => setEditing(null)} />}
    </>
  )
}

function Name({ p }: { p: AdminProductRow }) {
  return (
    <span className="flex flex-col gap-0.5">
      <span className="flex flex-wrap items-center gap-1.5">
        <b className={cn(!p.isActive && 'text-faint line-through')}>{p.name}</b>
        {p.needsReview && <Tag tone="warn">to review</Tag>}
        {!p.isActive && <Tag tone="muted">disabled</Tag>}
        {p.low && p.isActive && <Tag tone="danger">low</Tag>}
      </span>
      <small className="font-normal text-faint tabular-nums">{p.barcode || 'no barcode'}{p.nameLocal && ` · ${p.nameLocal}`}</small>
    </span>
  )
}
