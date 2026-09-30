import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminProduct, AdminSupplierRow, PurchaseRequest, PurchaseRow, ReceiveResult, StockLine, TillProduct } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { suppliersQuery } from '@/features/counter/queries'
import { Seg } from '@/features/till/bits'
import { configQuery, errMsg } from '@/features/till/api'
import { printMoneySlip } from '@/features/till/print'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { LineEditor } from '../LineEditor'
import { type Method, MoneySourcePicker } from '../MoneySource'
import { purchaseQuery } from '../queries'
import { useFormKeys } from '../useFormKeys'
import { SupplierForm } from '../suppliers/SupplierForm'
import { billTotals, type Current } from '../totals'

export type EntryMode = 'order' | 'edit' | 'receive-now' | 'receive'
const TITLE: Record<EntryMode, string> = { order: 'New purchase order', edit: 'Edit order', 'receive-now': 'Receive a delivery', receive: 'Receive order' }
const plusDays = (n: number) => new Date(Date.now() + n * 86_400_000).toISOString().slice(0, 10)

/** One screen for every way goods are bought (karots-pos purchase entry +
 * receive): place or edit an order (no stock yet), receive a delivery that came
 * without one, or receive an order — over or short, with forgotten items, the
 * rest kept on order, and pay on the spot. */
export function PurchaseEntry({ mode, id = 0, supplierId = 0 }: { mode: EntryMode; id?: number; supplierId?: number }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const go = useNavigate()
  const receiving = mode === 'receive' || mode === 'receive-now'
  const existing = useQuery({ ...purchaseQuery(id), enabled: id > 0 }).data
  const sups = useQuery(suppliersQuery('')).data ?? []
  const [sup, setSup] = useState(supplierId)
  const [lines, setLines] = useState<StockLine[] | null>(null)
  const [current, setCurrent] = useState<Record<number, Current>>({})
  const [invoice, setInvoice] = useState('')
  const [due, setDue] = useState('')
  const [expected, setExpected] = useState('')
  const [notes, setNotes] = useState('')
  const [disc, setDisc] = useState('')
  const [discType, setDiscType] = useState<'fixed' | 'percent'>('fixed')
  const [keepRest, setKeepRest] = useState(true)
  const [pay, setPay] = useState('')
  const [method, setMethod] = useState<Method>('cash')
  const [source, setSource] = useState('')
  const [adding, setAdding] = useState(false)
  const [busy, setBusy] = useState(false)

  // Load a draft once: its lines (receiving starts at what was ordered) and header.
  useEffect(() => {
    if (!existing || lines) return
    setSup(existing.supplierId)
    setLines(existing.lines.map((l) => ({ ...l, quantity: receiving ? l.ordered || l.quantity : l.quantity })))
    setCurrent(Object.fromEntries(existing.lines.map((l) => [l.productId, { cost: l.curCost, sell: l.curSell }])))
    setExpected(existing.expectedDate)
    setNotes(existing.notes)
    setDisc(existing.discountValue)
    setDiscType(existing.discountType === 'percent' ? 'percent' : 'fixed')
  }, [existing, lines, receiving])
  const shown = lines ?? []
  const supplier = sups.find((x) => x.id === sup)
  // Due date follows the supplier's credit days until typed over.
  const dueDate = due || (supplier?.creditDays ? plusDays(supplier.creditDays) : '')
  const { net, bill, total } = billTotals(shown, disc, discType)
  const short = mode === 'receive' && shown.some((l) => l.ordered && num(l.quantity) < num(l.ordered))

  // A product added from search: prefill its current cost, remember cost + price
  // for the margin guard, and take its usual supplier when none is picked yet.
  async function onAdd(p: TillProduct): Promise<Partial<StockLine>> {
    const a = await qc.fetchQuery({ queryKey: ['admin', 'product', p.id], queryFn: () => api<AdminProduct>(`/admin/products/${p.id}`) })
    setCurrent((c) => ({ ...c, [p.id]: { cost: a.costPrice, sell: a.sellingPrice } }))
    if (!sup && a.supplierId) setSup(a.supplierId)
    return { costPrice: num(a.costPrice) > 0 ? a.costPrice : '' }
  }

  async function save() {
    // A line that didn't come at all (0) is left off; the server keeps it on order.
    const sent = receiving ? shown.filter((l) => num(l.quantity) > 0) : shown
    if (!sup) return void toast.error('Pick the supplier')
    if (!sent.length) return void toast.error(receiving ? 'Enter what came in' : 'Add what you want to order')
    if (receiving && sent.some((l) => !(num(l.costPrice) > 0))) return void toast.error('Every line that came in needs its cost price')
    setBusy(true)
    try {
      const body: PurchaseRequest = { supplierId: sup, invoiceNo: invoice, notes, expectedDate: expected, dueDate: receiving ? dueDate : '', discount: disc, discountType: discType,
        lines: sent, keepRemainder: keepRest && short, pay: { amount: receiving ? pay : '', method, source } }
      let to = id
      if (mode === 'order') to = (await api<PurchaseRow>('/admin/purchases', { json: body })).id
      else if (mode === 'edit') await api(`/admin/purchases/${id}`, { method: 'PUT', json: body })
      else {
        const r = await api<ReceiveResult>(mode === 'receive' ? `/admin/purchases/${id}/receive` : '/admin/purchases/receive', { json: body })
        to = r.purchaseId
        const rid = r.receiptId
        const msg = `Received ${money(sym, r.total)} of stock${num(r.paid) > 0 ? ` · paid ${money(sym, r.paid)}` : ' on credit'}`
        toast.success(msg, rid ? { duration: 10_000, action: { label: 'Print receipt', onClick: () => void printMoneySlip(rid) } } : undefined)
      }
      if (!receiving) toast.success(mode === 'order' ? `Order #${to} placed with ${supplier?.name ?? 'the supplier'}` : 'Order updated')
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      void go({ to: '/admin/purchases/$id', params: { id: String(to) } })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  useFormKeys(() => document.querySelector<HTMLInputElement>('[aria-label="Add a product"]')?.focus(), () => { if (!busy && shown.length && sup) void save() })
  const submitLabel = mode === 'order' ? `Place order · ${money(sym, total)}` : mode === 'edit' ? 'Save order' : `Receive ${money(sym, total)} of stock`
  return (
    <>
      <Link to={id ? '/admin/purchases/$id' : '/admin/purchases'} params={{ id: String(id) }} className="mb-2 inline-flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground">
        <ArrowLeft size={14} /> {id ? `Order #${id}` : 'Purchases'}
      </Link>
      <h1 className="mb-4 font-display text-[24px] font-bold tracking-tight md:text-[28px]">{TITLE[mode]}{mode === 'receive' && supplier && ` from ${supplier.name}`}</h1>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
        <section className={cn(SURFACE, 'flex min-h-[420px] flex-col pt-4')}>
          <LineEditor lines={shown} onChange={setLines} withPrices sym={sym} fromOrder={id > 0 && !existing} onAdd={onAdd} current={current} />
        </section>
        <aside className={cn(SURFACE, 'flex flex-col gap-3.5 self-start p-4')}>
          <Field label="Supplier">
            <div className="flex gap-1.5">
              <select aria-label="Supplier" value={sup} disabled={id > 0} onChange={(e) => setSup(Number(e.target.value))} className={cn(INPUT, 'min-w-0 flex-1 appearance-auto disabled:opacity-70')}>
                <option value={0}>— pick a supplier —</option>
                {sups.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
              </select>
              {!id && <Button type="button" variant="outline" className="h-[var(--control-h)] px-3" onClick={() => setAdding(true)}>New</Button>}
            </div>
          </Field>
          {receiving ? (
            <div className="grid grid-cols-2 gap-2">
              <Field label="Their invoice no"><input aria-label="Their invoice no" value={invoice} onChange={(e) => setInvoice(e.target.value)} className={INPUT} placeholder="Optional" /></Field>
              <Field label="Pay by"><input type="date" aria-label="Pay by" value={dueDate} onChange={(e) => setDue(e.target.value)} className={INPUT} /></Field>
            </div>
          ) : (
            <Field label="Expected on"><input type="date" aria-label="Expected on" value={expected} onChange={(e) => setExpected(e.target.value)} className={INPUT} /></Field>
          )}
          <Field label="Bill discount">
            <div className="flex items-center gap-1.5">
              <input aria-label="Bill discount" inputMode="decimal" placeholder="0" value={disc} onChange={(e) => setDisc(e.target.value.replace(/[^\d.]/g, ''))} className={cn(INPUT, 'min-w-0 flex-1 text-right tabular-nums')} />
              <Seg label="Bill discount type" value={discType} onChange={setDiscType} options={[['fixed', sym], ['percent', '%']]} />
            </div>
          </Field>
          <Field label="Notes"><input aria-label="Notes" value={notes} onChange={(e) => setNotes(e.target.value)} className={INPUT} placeholder={receiving ? 'Optional' : 'For the supplier (optional)'} /></Field>
          <dl className="flex flex-col gap-1 border-t border-border pt-3 text-sm tabular-nums">
            <div className="flex justify-between text-muted-foreground"><dt>{shown.length} line{shown.length === 1 ? '' : 's'}</dt><dd>{money(sym, net)}</dd></div>
            {bill > 0 && <div className="flex justify-between text-muted-foreground"><dt>Bill discount</dt><dd>−{money(sym, bill)}</dd></div>}
            <div className="flex justify-between text-base font-bold"><dt>Total</dt><dd>{money(sym, total)}</dd></div>
          </dl>
          {receiving && (
            <div className="flex flex-col gap-2 rounded-[var(--radius-sm)] bg-muted p-3">
              <Field label="Pay now">
                <div className="flex gap-1.5">
                  <input aria-label="Pay now" inputMode="decimal" placeholder="Blank = on credit" value={pay} onChange={(e) => setPay(e.target.value.replace(/[^\d.]/g, ''))} className={cn(INPUT, 'min-w-0 flex-1 bg-card tabular-nums')} />
                  <button type="button" onClick={() => setPay(total.toFixed(2))} className="rounded-[var(--radius-sm)] border border-border bg-card px-3 text-[13px] font-bold">All</button>
                </div>
              </Field>
              {num(pay) > 0 && (
                <>
                  <Seg label="Pay by method" value={method} onChange={(m) => { setMethod(m); setSource('') }} options={[['cash', 'Cash'], ['card', 'Card'], ['online', 'Online']]} />
                  {method !== 'card' && <MoneySourcePicker method={method} value={source} onChange={setSource} sym={sym} label={method === 'cash' ? 'Pay the cash from' : 'Paid from (optional)'} />}
                </>
              )}
            </div>
          )}
          {short && (
            <label className="flex items-center gap-2 text-[13px] text-muted-foreground">
              <input type="checkbox" className="accent-[var(--brand)]" checked={keepRest} onChange={(e) => setKeepRest(e.target.checked)} />
              Keep what didn't come on a new order
            </label>
          )}
          <Button size="lg" disabled={busy || !shown.length || !sup} onClick={() => void save()} className="h-12 text-[15px] font-bold">{submitLabel}</Button>
          <small className="-mt-1 hidden text-center text-faint md:block">F4 add a product · F10 save</small>
        </aside>
      </div>
      {adding && <SupplierForm onClose={() => setAdding(false)} onSaved={(r: AdminSupplierRow) => { void qc.invalidateQueries({ queryKey: ['till'] }); setSup(r.id) }} />}
    </>
  )
}
