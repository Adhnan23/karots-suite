import { keepPreviousData, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Search, ShoppingCart, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { IDsResult, ReorderRow, ReorderRequest, SupplierRow } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { suppliersQuery } from '@/features/counter/queries'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { reorderQuery } from '../queries'
import { type Cart, cartSummary, loadCart, saveCart } from './cart'

const route = getRouteApi('/_authed/admin/reorder')
const CELL = cn(INPUT, 'h-10 px-2 text-sm')

function SupplierSelect({ value, onChange, sups, label }: { value: number; onChange: (id: number) => void; sups: SupplierRow[]; label: string }) {
  return (
    <select aria-label={label} value={value} onChange={(e) => onChange(Number(e.target.value))} className={cn(CELL, 'appearance-auto', !value && 'border-warning')}>
      <option value={0}>— supplier —</option>
      {sups.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
    </select>
  )
}

/** Reorder (karots-pos low-stock PO builder): everything at or under its
 * reorder level with a demand-based quantity. Tick what to buy across as many
 * searches as you like, then make one purchase order per supplier. */
export function ReorderPage() {
  const s = route.useSearch()
  const f = { q: s.q ?? '', category: s.category ?? 0, supplier: s.supplier ?? 0, page: s.page ?? 1 }
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const sups = useQuery(suppliersQuery('')).data ?? []
  const qc = useQueryClient()
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q, 250)
  useEffect(() => { if (dq !== f.q) void navigate({ search: (x) => ({ ...x, q: dq, page: 1 }), replace: true }) }, [dq, f.q, navigate])
  const data = useQuery({ ...reorderQuery(f), placeholderData: keepPreviousData }).data
  const [cart, setCartState] = useState<Cart>(loadCart)
  const [edits, setEdits] = useState<Record<number, { qty: string; supplierId: number }>>({})
  const [review, setReview] = useState(false)
  const [busy, setBusy] = useState(false)
  const setCart = (c: Cart) => { setCartState(c); saveCart(c) }
  const set = (patch: Partial<typeof f>) => void navigate({ search: (x) => ({ ...x, page: 1, ...patch }) })
  const sum = cartSummary(cart)

  const lineOf = (r: ReorderRow) => cart[r.productId] ?? { productId: r.productId, name: r.name, unitAbbr: r.unitAbbr, costPrice: r.cost,
    quantity: edits[r.productId]?.qty ?? r.suggested, supplierId: edits[r.productId]?.supplierId ?? r.supplierId }
  const change = (r: ReorderRow, patch: { quantity?: string; supplierId?: number }) => {
    const l = { ...lineOf(r), ...patch }
    if (cart[r.productId]) setCart({ ...cart, [r.productId]: l })
    else setEdits({ ...edits, [r.productId]: { qty: l.quantity, supplierId: l.supplierId } })
  }
  const tick = (r: ReorderRow, on: boolean) => {
    const next = { ...cart }
    if (on) next[r.productId] = lineOf(r)
    else delete next[r.productId]
    setCart(next)
  }

  async function build() {
    const lines = Object.values(cart).filter((i) => num(i.quantity) > 0)
    if (!lines.length) return void toast.error('Tick at least one item to order')
    if (lines.some((i) => !i.supplierId)) { setReview(true); return void toast.error('Choose a supplier for every ticked item') }
    setBusy(true)
    try {
      const body: ReorderRequest = { lines: lines.map(({ productId, supplierId, quantity, costPrice }) => ({ productId, supplierId, quantity, costPrice })) }
      const r = await api<IDsResult>('/admin/purchases/draft', { json: body })
      setCart({})
      setReview(false)
      toast.success(`${r.ids.length} purchase order${r.ids.length === 1 ? '' : 's'} created`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      window.open(`/print/po?ids=${r.ids.join(',')}`, '_blank')
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <PageHead title="Reorder" sub="Everything at or under its reorder level, with how much to buy. Tick items, then make the orders." />
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <div className="relative md:w-72">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search products" placeholder="Name or barcode" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        <div className="md:w-60"><CategoryPicker label="Filter by category" noneLabel="All categories" value={f.category} onChange={(category) => set({ category })} /></div>
        <select aria-label="Usual supplier" value={f.supplier} onChange={(e) => set({ supplier: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto md:w-56')}>
          <option value={0}>Any usual supplier</option>
          {sups.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
        </select>
      </div>
      <section className={cn(SURFACE, 'mb-24 overflow-hidden')}>
        {!data || data.items.length === 0 ? (
          <EmptyState title={!data ? 'Loading…' : f.q || f.category || f.supplier ? 'Nothing matches' : 'Nothing is running low'}>
            {data && !f.q && 'Set a reorder level on products to see them here when they run low.'}
          </EmptyState>
        ) : (
          <ul aria-label="Running low" className="divide-y divide-border">
            {data.items.map((r) => {
              const l = lineOf(r)
              const on = !!cart[r.productId]
              return (
                <li key={r.productId} className={cn('grid grid-cols-[28px_minmax(0,1fr)] items-center gap-x-3 gap-y-2 px-4 py-3 md:grid-cols-[28px_minmax(0,1fr)_110px_220px]', on && 'bg-brand-soft/40')}>
                  <input type="checkbox" aria-label={`Order ${r.name}`} checked={on} onChange={(e) => tick(r, e.target.checked)} className="size-5 accent-[var(--brand)]" />
                  <span className="min-w-0">
                    <b className="block truncate">{r.name}</b>
                    <small className="block text-faint tabular-nums">
                      <span className="font-semibold text-danger">{qtyText(r.onHand)} {r.unitAbbr} left</span> · reorder at {r.reorderLevel} · cost {money(sym, r.cost)}
                    </small>
                    <small className="block text-faint tabular-nums">
                      {r.fromSales ? 'from sales' : 'no recent sales — 2× reorder level'}{r.soldWeek && ` · ${r.soldWeek} this week`}{r.soldMonth && ` · ${r.soldMonth} this month`}{r.soldLastYear && ` · ${r.soldLastYear} same time last year`}
                    </small>
                  </span>
                  <label className="col-start-2 flex items-center gap-2 text-[13px] text-muted-foreground md:col-start-auto">
                    <span className="md:hidden">Order</span>
                    <input aria-label={`Quantity of ${r.name}`} inputMode="decimal" value={l.quantity} onChange={(e) => change(r, { quantity: e.target.value.replace(/[^\d.]/g, '') })} className={cn(CELL, 'w-24 text-right font-bold tabular-nums')} />
                  </label>
                  <div className="col-start-2 md:col-start-auto"><SupplierSelect label={`Supplier for ${r.name}`} value={l.supplierId} onChange={(supplierId) => change(r, { supplierId })} sups={sups} /></div>
                </li>
              )
            })}
          </ul>
        )}
        {data && <Pager page={data.page} pageSize={data.pageSize} total={data.total} onPage={(page) => void navigate({ search: (x) => ({ ...x, page }) })} />}
      </section>

      {sum.items > 0 && (
        <div className={cn(SURFACE, 'fixed inset-x-3 bottom-24 z-30 mx-auto flex max-w-xl items-center gap-3 p-3 md:bottom-28')}>
          <ShoppingCart size={20} className="text-brand" />
          <button type="button" onClick={() => setReview(true)} className="min-w-0 flex-1 text-left text-sm">
            <b>{sum.items} item{sum.items === 1 ? '' : 's'}</b> · {sum.suppliers} order{sum.suppliers === 1 ? '' : 's'}{sum.noSupplier > 0 && <span className="text-warning"> · {sum.noSupplier} need a supplier</span>}
            <small className="block text-faint">Review</small>
          </button>
          <Button className="h-10 px-4 font-bold" disabled={busy} onClick={() => void build()}>Create orders</Button>
        </div>
      )}
      {review && (
        <FormSheet open onOpenChange={setReview} title="To order" submit={`Create ${sum.suppliers || ''} order${sum.suppliers === 1 ? '' : 's'}`} busy={busy || sum.items === 0} onSubmit={() => void build()}
          extra={<Button type="button" variant="ghost" className="h-10 px-3 text-danger" onClick={() => { setCart({}); setReview(false) }}>Clear all</Button>}>
          <ul className="flex flex-col gap-2">
            {Object.values(cart).map((i) => (
              <li key={i.productId} className="grid grid-cols-[minmax(0,1fr)_80px_24px] items-center gap-2 text-sm">
                <span className="min-w-0"><b className="block truncate">{i.name}</b>
                  <SupplierSelect label={`Supplier for ${i.name}`} value={i.supplierId} onChange={(supplierId) => setCart({ ...cart, [i.productId]: { ...i, supplierId } })} sups={sups} /></span>
                <input aria-label={`Quantity of ${i.name}`} inputMode="decimal" value={i.quantity} onChange={(e) => setCart({ ...cart, [i.productId]: { ...i, quantity: e.target.value.replace(/[^\d.]/g, '') } })} className={cn(CELL, 'text-right tabular-nums')} />
                <button type="button" aria-label={`Remove ${i.name}`} onClick={() => { const n = { ...cart }; delete n[i.productId]; setCart(n) }} className="text-faint hover:text-danger"><X size={16} /></button>
              </li>
            ))}
          </ul>
        </FormSheet>
      )}
    </>
  )
}
