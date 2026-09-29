import { keepPreviousData, queryOptions, useQuery, useQueryClient } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'
import { Download, Search, Upload } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { CountLine, ImportSummary, StockTakeList, StockTakeResult, StockTakeRow } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { PickerShell } from '@/components/admin/PickerShell'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { supplierOptionsQuery } from '@/features/admin/products/queries'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { num, qtyText } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'

const route = getRouteApi('/_authed/admin/stock/take')
type F = { q: string; category: number; supplier: number; page: number }
const takeQuery = (f: F) => queryOptions({
  queryKey: ['admin', 'stock-take', f],
  queryFn: () => api<StockTakeList>(`/admin/stock/take?q=${encodeURIComponent(f.q)}&category=${f.category}&supplier=${f.supplier}&page=${f.page}`),
})
const CELL = cn(INPUT, 'h-10 px-2.5 text-right tabular-nums')
type Edit = Omit<CountLine, 'productId'>
const EMPTY: Edit = { counted: '', cost: '', selling: '', wholesale: '' }

/** Count and reconcile on-hand (karots-pos Stock-take): type what's on the
 * shelf, optionally correct cost and prices, save. Or do it in a spreadsheet. */
export function StockTakePage() {
  const s = route.useSearch()
  const navigate = route.useNavigate()
  const f: F = { q: s.q ?? '', category: s.category ?? 0, supplier: s.supplier ?? 0, page: s.page ?? 1 }
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q, 250)
  useEffect(() => { if (dq !== f.q) void navigate({ search: (x) => ({ ...x, q: dq, page: 1 }), replace: true }) }, [dq, f.q, navigate])
  const data = useQuery({ ...takeQuery(f), placeholderData: keepPreviousData }).data
  const suppliers = useQuery(supplierOptionsQuery).data ?? []
  const qc = useQueryClient()
  const [edits, setEdits] = useState<Record<number, Edit>>({})
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<ImportSummary | null>(null)
  const file = useRef<HTMLInputElement>(null)
  const list = useRef<HTMLDivElement>(null)
  const dirty = Object.entries(edits).filter(([, e]) => e.counted || e.cost || e.selling || e.wholesale)
  // Typed counts are kept across filters and pages (keyed by product), so one Save covers the whole walk round the shop.
  const set = (patch: Partial<F>) => void navigate({ search: (x) => ({ ...x, page: 1, ...patch }) })
  const edit = (id: number, patch: Partial<Edit>) => setEdits((x) => ({ ...x, [id]: { ...(x[id] ?? EMPTY), ...patch } }))

  async function save() {
    setBusy(true)
    try {
      const r = await api<StockTakeResult>('/admin/stock/take', { json: { lines: dirty.map(([id, e]) => ({ productId: Number(id), ...e })) } })
      toast.success(r.saved ? `${r.saved} ${r.saved === 1 ? 'product' : 'products'} updated` : 'Everything already matched')
      setEdits({})
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (e) {
      toast.error(errMsg(e))
      void qc.invalidateQueries({ queryKey: ['admin', 'stock-take'] })
    } finally {
      setBusy(false)
    }
  }
  async function upload(fl: File) {
    const fd = new FormData()
    fd.append('file', fl)
    try {
      const res = await fetch('/api/v1/admin/stock/take/import', { method: 'POST', body: fd, credentials: 'same-origin' })
      const body = await res.json()
      if (!res.ok) throw new ApiError(res.status, body?.error?.code, body?.error?.message ?? res.statusText, '')
      setResult(body as ImportSummary)
      void qc.invalidateQueries({ queryKey: ['admin'] })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  // Enter in a count jumps to the next row's count: count down the shelf without the mouse.
  const nextCount = (i: number) => (list.current?.querySelectorAll<HTMLInputElement>('input[data-count]')[i + 1])?.focus()

  return (
    <>
      <PageHead title="Stock-take" sub="Type what’s really on the shelf. Blank rows stay as they are."
        actions={<>
          <DownloadMenu />
          <Button variant="outline" className="h-10 gap-1.5 px-3" onClick={() => file.current?.click()}><Upload size={15} /> Import count</Button>
          <input ref={file} type="file" hidden accept=".csv,.xlsx,.ods" onChange={(e) => { const fl = e.target.files?.[0]; e.target.value = ''; if (fl) void upload(fl) }} />
        </>} />
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <div className="relative md:w-72">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search products" placeholder="Name or barcode" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        <div className="md:w-60"><CategoryPicker label="Filter by category" noneLabel="All categories" value={f.category} onChange={(category) => set({ category })} /></div>
        <select aria-label="Supplier" value={f.supplier} onChange={(e) => set({ supplier: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto md:w-52')}>
          <option value={0}>Every supplier</option>
          {suppliers.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
        </select>
      </div>
      <div ref={list} className={cn(SURFACE, 'overflow-hidden')}>
        <div className="hidden grid-cols-[minmax(0,2fr)_90px_110px_110px_110px_110px] gap-2 border-b border-border px-4 py-2.5 text-[12px] font-bold uppercase tracking-wide text-faint md:grid">
          <span>Product</span><span className="text-right">On hand</span><span className="text-right">Counted</span><span className="text-right">Cost</span><span className="text-right">Selling</span><span className="text-right">Wholesale</span>
        </div>
        {!data?.items.length ? <EmptyState title={data ? 'Nothing matches' : 'Loading…'} /> : (
          <ul aria-label="Products to count" className="divide-y divide-border">
            {data.items.map((p, i) => <Row key={p.id} p={p} e={edits[p.id] ?? EMPTY} onEdit={(x) => edit(p.id, x)} onNext={() => nextCount(i)} />)}
          </ul>
        )}
        {data && <Pager page={data.page} pageSize={data.pageSize} total={data.total} onPage={(page) => set({ page })} />}
      </div>
      {dirty.length > 0 && (
        <div className="sticky bottom-[96px] z-10 mt-3 flex items-center gap-3 rounded-[var(--radius)] border border-border bg-popover p-3 shadow-[var(--shadow-float)] md:bottom-[100px]">
          <span className="flex-1 text-sm"><b>{dirty.length}</b> {dirty.length === 1 ? 'product' : 'products'} changed</span>
          <Button variant="ghost" className="h-10 px-3" onClick={() => setEdits({})}>Discard</Button>
          <Button className="h-10 px-5 font-bold" disabled={busy} onClick={save}>Save counts</Button>
        </div>
      )}
      {result && <ImportResult r={result} onClose={() => setResult(null)} />}
    </>
  )
}

function Row({ p, e, onEdit, onNext }: { p: StockTakeRow; e: Edit; onEdit: (x: Partial<Edit>) => void; onNext: () => void }) {
  const delta = e.counted === '' ? 0 : num(e.counted) - num(p.stockQty)
  const clean = (v: string) => v.replace(/[^\d.]/g, '')
  return (
    <li className={cn('grid grid-cols-2 gap-2 px-4 py-2.5 md:grid-cols-[minmax(0,2fr)_90px_110px_110px_110px_110px] md:items-center', (e.counted || e.cost || e.selling || e.wholesale) && 'bg-brand-soft')}>
      <span className="col-span-2 min-w-0 md:col-span-1">
        <span className="flex flex-wrap items-center gap-1.5"><b className="truncate">{p.name}</b>{p.multiPrice && <Tag tone="warn">2+ lot prices</Tag>}</span>
        <small className="text-faint tabular-nums">{p.barcode || p.categoryName}
          {p.multiPrice && <> · a change comes off the oldest lot — <Link to="/admin/stock" search={{ q: p.name }} className="font-semibold text-foreground underline">adjust it there</Link> to pick the lot</>}</small>
      </span>
      <span className="self-center text-[13px] md:text-right md:text-sm">
        <span className="text-faint md:hidden">On hand </span><b className="tabular-nums">{qtyText(p.stockQty)}</b> <small className="text-faint">{p.unitAbbr}</small>
      </span>
      <label className="relative">
        {/* aria-label on the input: the ±delta below sits inside this label and would otherwise join its name. */}
        <input data-count aria-label={`Counted ${p.name}`} inputMode="decimal" value={e.counted} placeholder={qtyText(p.stockQty)} onChange={(x) => onEdit({ counted: clean(x.target.value) })}
          onKeyDown={(x) => { if (x.key === 'Enter') { x.preventDefault(); onNext() } }} className={cn(CELL, 'font-bold')} />
        {delta !== 0 && <small className={cn('absolute -bottom-3.5 right-1 text-[10.5px] font-bold', delta > 0 ? 'text-success' : 'text-danger')}>{delta > 0 ? '+' : '−'}{qtyText(Math.abs(delta))}</small>}
      </label>
      <div className="col-span-2 grid grid-cols-3 gap-2 md:contents">
        {([['cost', 'Cost', p.costPrice], ['selling', 'Selling', p.sellingPrice], ['wholesale', 'Wholesale', num(p.wholesalePrice) ? p.wholesalePrice : '—']] as const).map(([k, l, ph]) => (
          <label key={k} className="flex flex-col gap-0.5">
            <small className="text-[11px] font-semibold text-faint md:hidden">{l}</small>
            <input aria-label={`${l === 'Cost' ? 'Cost' : `${l} price`} of ${p.name}`} inputMode="decimal" value={e[k]} placeholder={ph}
              onChange={(x) => onEdit({ [k]: clean(x.target.value) })} className={CELL} />
          </label>
        ))}
      </div>
    </li>
  )
}

function DownloadMenu() {
  const href = (fmt: string) => `/api/v1/admin/stock/take/sheet?format=${fmt}`
  return (
    <details className="relative">
      <summary className="flex h-10 cursor-pointer list-none items-center gap-1.5 rounded-lg border border-border bg-background px-3 text-sm font-medium hover:bg-muted"><Download size={15} /> Count sheet</summary>
      <div className="absolute right-0 z-20 mt-1 flex w-44 flex-col rounded-[var(--radius-sm)] border border-border bg-popover p-1 shadow-[var(--shadow-float)]">
        {[['csv', 'CSV'], ['xlsx', 'Excel (.xlsx)'], ['ods', 'LibreOffice (.ods)']].map(([f, l]) => (
          <a key={f} href={href(f)} download className="rounded-md px-2.5 py-2 text-sm hover:bg-muted">{l}</a>
        ))}
      </div>
    </details>
  )
}

function ImportResult({ r, onClose }: { r: ImportSummary; onClose: () => void }) {
  return (
    <PickerShell fit open onOpenChange={(o) => !o && onClose()} title="Count imported">
      <div className="grid grid-cols-3 gap-2 text-center">
        {[['Updated', r.updated, 'text-success'], ['Already right', r.unchanged, ''], ['Skipped', r.skipped, r.skipped ? 'text-danger' : '']].map(([l, n, tone]) => (
          <div key={l as string} className="rounded-[var(--radius-sm)] bg-muted p-2.5"><b className={cn('block font-display text-2xl', tone as string)}>{n}</b><small className="text-muted-foreground">{l}</small></div>
        ))}
      </div>
      {r.errors.length > 0 && (
        <ul className="max-h-60 overflow-auto rounded-[var(--radius-sm)] border border-border text-sm">
          {r.errors.map((e) => <li key={e.line} className="border-b border-border px-3 py-1.5 last:border-0"><b>Row {e.line}</b> — {e.message}</li>)}
        </ul>
      )}
      {r.notes.map((n) => <p key={n} className="text-sm text-muted-foreground">{n}</p>)}
      <Button className="h-10 font-bold" onClick={onClose}>Done</Button>
    </PickerShell>
  )
}
