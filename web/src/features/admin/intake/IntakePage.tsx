import { keepPreviousData, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { PackagePlus, Printer, RotateCcw, Search, Undo2, Wand2 } from 'lucide-react'
import { useRef, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminProductRow, DetailRow, GeneratedBarcode, IntakeItem } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { editPrice, markupOf, type Prices } from '@/features/admin/products/price'
import { productsQuery, supplierOptionsQuery } from '@/features/admin/products/queries'
import { SameAsExisting } from '@/features/admin/products/SameAsExisting'
import { unitsQuery } from '@/features/admin/units/UnitsPage'
import { extraBody, type Extras, ProductAssists, ProductExtras } from '@/features/admin/products/ProductExtras'
import { configQuery, errMsg } from '@/features/till/api'
import { hasBarcode } from '@/lib/barcode'
import { money, num, qtyText } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { matchBatch } from '@/features/admin/stock/batches'
import { lotsQuery } from '@/features/admin/stock/queries'
import { BatchChoice } from './BatchChoice'
import { initialLabels, LabelBlock, labelRequest, type LabelOpts } from './LabelBlock'

type Mode = { kind: 'restock'; p: AdminProductRow; scanned?: string } | { kind: 'new'; name: string; barcode: string } | null
const clean = (v: string) => v.replace(/[^\d.]/g, '')
const looksScanned = (s: string) => /^\d{6,}$/.test(s.trim())

/** Stock intake (karots-pos): one box — scan or type — then restock what exists
 * or create what doesn't, set the count and print its labels in the same go. */
export function IntakePage() {
  const cfg = useSuspenseQuery(configQuery).data
  const qc = useQueryClient()
  const [q, setQ] = useState('')
  const dq = useDebounced(q.trim(), 200)
  const hits = useQuery({ ...productsQuery({ q: dq, category: 0, low: false, disabled: false, page: 1 }), enabled: dq.length > 0, placeholderData: keepPreviousData }).data?.items ?? []
  const [mode, setMode] = useState<Mode>(null)
  const [session, setSession] = useState<IntakeItem[]>([])
  const [labels, setLabels] = useState<LabelOpts>(() => initialLabels(cfg.labelSize))
  const search = useRef<HTMLInputElement>(null)

  const back = () => { setMode(null); setQ(''); search.current?.focus() }
  // After a save: clear only what that item left behind. A scanner may already have typed the
  // next code (and opened it) while the save was in flight — that must survive.
  const done = (m: Mode, typed: string) => {
    setMode((cur) => (cur === m ? null : cur))
    setQ((cur) => (cur === typed ? '' : cur))
    search.current?.focus()
  }
  // Scanner Enter: an exact barcode (or the only hit) restocks it; anything else starts a new product.
  async function enter() {
    const term = q.trim()
    if (!term) return
    const found = (await qc.fetchQuery(productsQuery({ q: term, category: 0, low: false, disabled: false, page: 1 }))).items
    const exact = found.find((p) => hasBarcode(p, term)) ?? (found.length === 1 ? found[0] : undefined)
    setMode(exact ? { kind: 'restock', p: exact, scanned: hasBarcode(exact, term) ? term : undefined } : looksScanned(term) ? { kind: 'new', name: '', barcode: term } : { kind: 'new', name: term, barcode: '' })
  }
  async function printLabels(item: IntakeItem, fields: DetailRow[] = []) {
    const body = labelRequest(labels, { productId: item.productId, name: item.name, price: item.sellingPrice, barcode: item.barcode, qty: item.qty }, cfg.symbol, fields)
    try {
      await api('/admin/labels/send', { json: body })
      toast.success(`${body.count} ${body.count === 1 ? 'label' : 'labels'} sent to the printer`)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  function saved(item: IntakeItem, fields: DetailRow[] = []) {
    setSession((s) => [item, ...s])
    void qc.invalidateQueries({ queryKey: ['admin'] })
    void qc.invalidateQueries({ queryKey: ['till'] })
    if (labels.on) void printLabels(item, fields)
    setLabels((l) => ({ ...l, count: '' }))
  }
  async function undo(item: IntakeItem, i: number) {
    try {
      await api('/admin/inventory/intake/undo', { json: { productId: item.productId, qty: item.qty, kind: item.kind, batchId: item.batchId } })
      setSession((s) => s.filter((_, j) => j !== i))
      toast.success(item.kind === 'create' ? `${item.name} removed (find it under Show disabled)` : `${item.name}: −${qtyText(item.qty)}`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  const units = session.reduce((s, x) => s + num(x.qty), 0)

  return (
    <>
      <PageHead title="Stock intake" sub="Scan or type. Existing items get restocked, new ones created — count and labels in one go." />
      {/* Phone: search → form → session. Desktop: search and session left, the form right. */}
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)] lg:grid-rows-[auto_1fr] lg:items-start">
          <div className={cn(SURFACE, 'p-4 lg:col-start-1 lg:row-start-1')}>
            <div className="relative">
              <Search size={18} className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-faint" />
              <input ref={search} autoFocus aria-label="Scan or search" placeholder="Scan a barcode or type a name" value={q}
                onChange={(e) => { setQ(e.target.value); if (mode) setMode(null) }} onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void enter() } }}
                className={cn(INPUT, 'h-14 pl-11 text-lg')} />
            </div>
            {dq && !mode && (
              <ul aria-label="Matches" className="mt-2 flex flex-col">
                {hits.slice(0, 8).map((p) => (
                  <li key={p.id}>
                    <button type="button" onClick={() => setMode({ kind: 'restock', p })} className="flex w-full items-center gap-3 rounded-[var(--radius-sm)] px-2.5 py-2 text-left hover:bg-muted">
                      <span className="min-w-0 flex-1"><b className="block truncate text-sm">{p.name}</b><small className="text-faint tabular-nums">{p.barcode || p.categoryName}</small></span>
                      <small className="text-right tabular-nums text-muted-foreground">{qtyText(p.stockQty)} {p.unitAbbr}<br />{money(cfg.symbol, p.sellingPrice)}</small>
                    </button>
                  </li>
                ))}
                <li>
                  <button type="button" onClick={() => setMode(looksScanned(q) ? { kind: 'new', name: '', barcode: q.trim() } : { kind: 'new', name: q.trim(), barcode: '' })}
                    className="flex w-full items-center gap-2 rounded-[var(--radius-sm)] px-2.5 py-2.5 text-left text-sm font-bold text-foreground hover:bg-muted">
                    <PackagePlus size={17} /> New product “{q.trim()}”
                  </button>
                </li>
              </ul>
            )}
          </div>
          <section aria-label="Added this session" className={cn(SURFACE, 'order-last overflow-hidden lg:order-none lg:col-start-1 lg:row-start-2')}>
            <header className="flex items-center justify-between border-b border-border px-4 py-3">
              <b>Added this session</b>
              <small className="text-muted-foreground tabular-nums">{session.length} {session.length === 1 ? 'item' : 'items'} · {qtyText(units)} units</small>
            </header>
            {session.length === 0 ? <EmptyState title="Nothing yet">Each item you add shows here, with its labels and an undo.</EmptyState> : (
              <ul className="divide-y divide-border">
                {session.map((x, i) => (
                  <li key={`${x.productId}-${i}`} className="flex items-center gap-2 px-4 py-2.5">
                    <span className="min-w-0 flex-1">
                      <b className="block truncate text-sm">{x.name}</b>
                      <small className="text-faint tabular-nums">{x.kind === 'create' ? 'new' : 'restock'} · +{qtyText(x.qty)} → {qtyText(x.stockQty)} {x.unitAbbr} · {money(cfg.symbol, x.sellingPrice)}</small>
                    </span>
                    {x.barcode && <Button variant="ghost" className="h-9 gap-1 px-2.5" aria-label={`Reprint labels for ${x.name}`} onClick={() => printLabels(x)}><Printer size={15} /></Button>}
                    <Button variant="ghost" className="h-9 gap-1 px-2.5 text-danger" aria-label={`Undo ${x.name}`} onClick={() => undo(x, i)}><Undo2 size={15} /></Button>
                  </li>
                ))}
              </ul>
            )}
          </section>
        <div className={cn(SURFACE, 'p-5 lg:col-start-2 lg:row-span-2 lg:row-start-1')}>
          {mode?.kind === 'restock' ? <Restock key={mode.p.id} p={mode.p} scanned={mode.scanned} labels={labels} setLabels={setLabels} onSaved={(x, f) => { saved(x, f); done(mode, q) }} onCancel={back} />
            : mode?.kind === 'new' ? <NewItem key={`${mode.name}|${mode.barcode}`} init={mode} labels={labels} setLabels={setLabels} onSaved={(x) => { saved(x); setQ('') }} onCancel={back}
              onLinked={(p, code) => setMode({ kind: 'restock', p, scanned: code })} />
              : <EmptyState title="Scan or search to start">Enter on a scanned barcode opens the item straight away — or a new product if it isn’t in the catalog yet.</EmptyState>}
        </div>
      </div>
    </>
  )
}

function PriceRow({ prices, set, wholesale, setWholesale }: { prices: Prices; set: (f: keyof Prices, v: string) => void; wholesale: string; setWholesale: (v: string) => void }) {
  const cell = cn(INPUT, 'tabular-nums')
  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
      <Field label="Cost"><input aria-label="Cost" inputMode="decimal" value={prices.cost} onChange={(e) => set('cost', clean(e.target.value))} className={cell} placeholder="0.00" /></Field>
      <Field label="Markup ×"><input aria-label="Markup" inputMode="decimal" value={prices.markup} onChange={(e) => set('markup', clean(e.target.value))} className={cell} placeholder="1.25" /></Field>
      <Field label="Selling"><input aria-label="Selling" inputMode="decimal" value={prices.selling} onChange={(e) => set('selling', clean(e.target.value))} className={cn(cell, 'font-bold')} placeholder="0.00" /></Field>
      <Field label="Wholesale"><input aria-label="Wholesale" inputMode="decimal" value={wholesale} onChange={(e) => setWholesale(clean(e.target.value))} className={cell} placeholder="—" /></Field>
    </div>
  )
}

function Restock({ p, scanned, labels, setLabels, onSaved, onCancel }: { p: AdminProductRow; scanned?: string; labels: LabelOpts; setLabels: (l: LabelOpts) => void; onSaved: (x: IntakeItem, fields: DetailRow[]) => void; onCancel: () => void }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const fields = useQuery({ queryKey: ['till', 'label-fields', p.id], queryFn: () => api<DetailRow[]>(`/till/products/${p.id}/label-fields`) }).data ?? []
  const [qty, setQty] = useState('')
  const [prices, setPrices] = useState<Prices>({ cost: p.costPrice, markup: markupOf(p.costPrice, p.sellingPrice), selling: p.sellingPrice })
  const [wholesale, setWholesale] = useState(num(p.wholesalePrice) ? p.wholesalePrice : '')
  const [barcode, setBarcode] = useState(p.barcode)
  const [busy, setBusy] = useState(false)
  const lots = useQuery(lotsQuery(p.id)).data?.lots ?? []
  const [expiry, setExpiry] = useState('')
  // The batch it goes into: the matching one until the person picks by hand.
  const [picked, setPicked] = useState<number | null>(null)
  const auto = matchBatch(lots, { cost: prices.cost, selling: prices.selling, expiry }, p.sellingPrice)
  const batchId = picked ?? auto
  // A barcode-less item gets one minted and saved, so its label scans at the till (karots-pos genBarcode).
  async function generate() {
    setBusy(true)
    try {
      const code = (await api<GeneratedBarcode>('/admin/barcode/generate')).barcode
      await api(`/admin/products/${p.id}/barcode`, { json: { barcode: code } })
      setBarcode(code)
      toast.success(`Barcode ${code} saved`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }
  async function save(e: { preventDefault(): void }) {
    e.preventDefault()
    setBusy(true)
    try {
      const changed = (a: string, b: string) => (num(a) !== num(b) ? a : '')
      const item = await api<IntakeItem>('/admin/inventory/intake/restock', { json: {
        productId: p.id, quantity: qty, costPrice: changed(prices.cost, p.costPrice), sellingPrice: changed(prices.selling, p.sellingPrice), wholesalePrice: changed(wholesale, p.wholesalePrice), scanned: scanned ?? '',
        batchId, expiry: batchId ? '' : expiry,
      } })
      toast.success(`${p.name}: +${qtyText(qty)}`)
      onSaved(item, fields)
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <form onSubmit={save} autoComplete="off" className="flex flex-col gap-4">
      <div className="flex items-start gap-3">
        <span className="grid h-11 w-11 flex-none place-items-center rounded-[12px] bg-brand-soft"><RotateCcw size={20} /></span>
        <span className="min-w-0 flex-1">
          <small className="font-semibold text-muted-foreground">Restock</small>
          <b className="block truncate font-display text-xl">{p.name}</b>
          <small className="text-faint tabular-nums">{barcode || 'no barcode'} · {qtyText(p.stockQty)} {p.unitAbbr} on hand · {money(sym, p.sellingPrice)}</small>
        </span>
      </div>
      {!barcode && (
        <div className="flex flex-wrap items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-warning px-3 py-2 text-[13px]">
          No barcode — generate one so the printed label scans at the till.
          <Button type="button" variant="secondary" size="sm" disabled={busy} onClick={generate}><Wand2 size={14} /> Generate barcode</Button>
        </div>
      )}
      <Field label={`How many ${p.unitAbbr} came in`}>
        <input autoFocus required aria-label="Quantity to add" inputMode="decimal" value={qty} onChange={(e) => setQty(clean(e.target.value))} className={cn(INPUT, 'h-14 font-display text-2xl font-bold tabular-nums')} placeholder="0" />
      </Field>
      <PriceRow prices={prices} set={(f, v) => setPrices((x) => editPrice(x, f, v, true))} wholesale={wholesale} setWholesale={setWholesale} />
      {num(prices.selling) !== num(p.sellingPrice) && <p className="text-[12px] text-muted-foreground">The new price goes on the product and sticks to the stock you’re adding now; stock already on the shelf keeps its price.</p>}
      <Field label="Expiry" hint="Optional"><input aria-label="Expiry" type="date" value={expiry} onChange={(e) => setExpiry(e.target.value)} className={cn(INPUT, 'md:w-56')} /></Field>
      {lots.length > 0 && <BatchChoice lots={lots} value={batchId} auto={auto} unit={p.unitAbbr} sym={sym} productPrice={p.sellingPrice} onPick={setPicked} />}
      <LabelBlock opts={labels} set={setLabels} name={p.name} price={prices.selling} barcode={barcode} qty={qty} fields={fields} />
      <div className="flex gap-2">
        <Button type="button" variant="ghost" className="h-11 px-4" onClick={onCancel}>Cancel</Button>
        <Button type="submit" disabled={busy || !(num(qty) > 0)} className="h-11 flex-1 font-bold">{labels.on ? 'Add stock & print labels' : 'Add stock'}</Button>
      </div>
    </form>
  )
}

type Draft = { name: string; barcode: string; cost: string; markup: string; selling: string; wholesale: string; qty: string; expiry: string }
// Kept between new items (karots-pos rapid loop): category, unit, supplier and markup.
let sticky = { categoryId: 0, unitId: 0, supplierId: 0, markup: '1.4' }
let lastNew: Draft | null = null

function NewItem({ init, labels, setLabels, onSaved, onCancel, onLinked }: { init: { name: string; barcode: string }; labels: LabelOpts; setLabels: (l: LabelOpts) => void; onSaved: (x: IntakeItem) => void; onCancel: () => void; onLinked: (p: AdminProductRow, code: string) => void }) {
  const units = useQuery(unitsQuery).data ?? []
  const suppliers = useQuery(supplierOptionsQuery).data ?? []
  const blank = (name: string, barcode: string): Draft => ({ name, barcode, cost: '', markup: sticky.markup, selling: '', wholesale: '', qty: '', expiry: '' })
  const [d, setD] = useState<Draft>(() => blank(init.name, init.barcode))
  const [cat, setCat] = useState(sticky.categoryId)
  const [unit, setUnit] = useState(sticky.unitId)
  const [supplier, setSupplier] = useState(sticky.supplierId)
  const [busy, setBusy] = useState(false)
  const [extras, setExtras] = useState<Extras>({})
  const [round, setRound] = useState(0) // remounts plugin sections after each save
  const [like, setLike] = useState(lastNew)
  const nameRef = useRef<HTMLInputElement>(null)
  const qtyRef = useRef<HTMLInputElement>(null)
  const unitId = unit || units.find((u) => u.abbreviation === 'pcs')?.id || units[0]?.id || 0
  const prices: Prices = { cost: d.cost, markup: d.markup, selling: d.selling }
  const setPrice = (f: keyof Prices, v: string) => setD((x) => ({ ...x, ...editPrice({ cost: x.cost, markup: x.markup, selling: x.selling }, f, v, true) }))

  async function save(e: { preventDefault(): void }) {
    e.preventDefault()
    setBusy(true)
    try {
      const item = await api<IntakeItem>('/admin/inventory/intake/create', { json: { quantity: d.qty, expiry: d.expiry, product: {
        name: d.name, nameLocal: '', barcode: d.barcode, categoryId: cat, unitId, costPrice: d.cost, sellingPrice: d.selling, wholesalePrice: d.wholesale, taxRate: '',
        reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: supplier, repriceStock: false, dupSourceName: '', extra: extraBody(extras),
      } } })
      sticky = { categoryId: cat, unitId, supplierId: supplier, markup: d.markup }
      lastNew = d
      setLike(d)
      toast.success(`${item.name} added${num(d.qty) > 0 ? ` with ${qtyText(d.qty)}` : ''}`)
      onSaved(item)
      // Rapid loop: stay on New, keep category/unit/supplier/markup, clear the rest, back to the name.
      setD(blank('', ''))
      setExtras({})
      setRound((r) => r + 1)
      nameRef.current?.focus()
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <form onSubmit={save} autoComplete="off" className="flex flex-col gap-4">
      <div className="flex items-center gap-3">
        <span className="grid h-11 w-11 flex-none place-items-center rounded-[12px] bg-brand text-brand-foreground"><PackagePlus size={20} /></span>
        <b className="flex-1 font-display text-xl">New item</b>
        {like && (
          <Button type="button" variant="outline" className="h-9 gap-1.5 px-3" title="Refill from the last item — same name, prices and category; change what differs"
            onClick={() => { setD({ ...like, barcode: '', qty: '' }); nameRef.current?.focus() }}><RotateCcw size={14} /> Another like last</Button>
        )}
      </div>
      {init.barcode && d.barcode === init.barcode && <SameAsExisting code={init.barcode} source="intake" onLinked={(p) => onLinked(p, init.barcode)} />}
      <div className="flex flex-col gap-2">
        <Field label="Name"><input ref={nameRef} autoFocus={!init.name} required maxLength={150} name="in-name" value={d.name} onChange={(e) => setD({ ...d, name: e.target.value })} className={INPUT} /></Field>
        <ProductAssists key={round} name={d.name} barcode={d.barcode} where="intake" setExtras={setExtras}
          onFill={({ name, categoryId }) => { if (name) setD((x) => ({ ...x, name })); if (categoryId) setCat(categoryId) }} />
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Category"><CategoryPicker label="Category" allowCreate value={cat} onChange={setCat} /></Field>
        <Field label="Unit">
          <select aria-label="Unit" value={unitId} onChange={(e) => setUnit(Number(e.target.value))} className={cn(INPUT, 'appearance-auto')}>
            {units.map((u) => <option key={u.id} value={u.id}>{u.name} ({u.abbreviation})</option>)}
          </select>
        </Field>
      </div>
      <Field label="Supplier" hint="Optional">
        <select aria-label="Supplier" value={supplier} onChange={(e) => setSupplier(Number(e.target.value))} className={cn(INPUT, 'appearance-auto')}>
          <option value={0}>None</option>
          {suppliers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
      </Field>
      <PriceRow prices={prices} set={setPrice} wholesale={d.wholesale} setWholesale={(wholesale) => setD({ ...d, wholesale })} />
      <Field label="Barcode">
        <div className="flex gap-1.5">
          <input name="in-code" value={d.barcode} onChange={(e) => setD({ ...d, barcode: e.target.value.trim() })} placeholder="Scan or type"
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); qtyRef.current?.focus() } }} className={cn(INPUT, 'min-w-0 flex-1 tabular-nums')} />
          <Button type="button" variant="outline" disabled={!!d.barcode} className="h-[var(--control-h)] gap-1.5 px-3"
            onClick={async () => { try { const code = (await api<GeneratedBarcode>('/admin/barcode/generate')).barcode; setD((x) => ({ ...x, barcode: code })) } catch (err) { toast.error(errMsg(err)) } }}><Wand2 size={15} /> Generate</Button>
        </div>
      </Field>
      <Field label="Opening quantity">
        <input ref={qtyRef} autoFocus={!!init.name} aria-label="Opening quantity" inputMode="decimal" value={d.qty} onChange={(e) => setD({ ...d, qty: clean(e.target.value) })} className={cn(INPUT, 'h-14 font-display text-2xl font-bold tabular-nums')} placeholder="0" />
      </Field>
      <Field label="Expiry" hint="Optional"><input aria-label="Expiry" type="date" value={d.expiry} onChange={(e) => setD({ ...d, expiry: e.target.value })} className={cn(INPUT, 'md:w-56')} /></Field>
      <ProductExtras key={round} productId={0} sourceId={0} categoryId={cat} where="intake" extras={extras} setExtras={setExtras} />
      <LabelBlock opts={labels} set={setLabels} name={d.name} price={d.selling} barcode={d.barcode} qty={d.qty} />
      <div className="flex gap-2">
        <Button type="button" variant="ghost" className="h-11 px-4" onClick={onCancel}>Cancel</Button>
        <Button type="submit" disabled={busy || !d.name.trim() || !cat} className="h-11 flex-1 font-bold">{labels.on ? 'Create, stock & print labels' : 'Create & stock'}</Button>
      </div>
    </form>
  )
}
