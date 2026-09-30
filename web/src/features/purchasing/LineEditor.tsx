import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Search, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { IntakeProductRequest, StockLine, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { errMsg, searchQuery } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lineDiscount, lineGross, lineNet } from './totals'

const CELL = cn(FIELD, 'h-9 min-w-0 px-2 text-right tabular-nums')
const blank = (p: TillProduct, prices: boolean, cost = ''): StockLine => ({
  productId: p.id, name: p.name, ordered: '', quantity: '1', freeQty: '', costPrice: cost, sellingPrice: prices ? p.sellingPrice : '', expiryDate: '',
  discount: '', discountType: 'fixed',
})
const dec = (v: string) => v.replace(/[^\d.]/g, '')

/** The delivery / order lines: scan or search to add, type the numbers in place. */
export function LineEditor({ lines, onChange, withPrices, sym, fromOrder }: {
  lines: StockLine[]; onChange: (l: StockLine[]) => void; withPrices: boolean; sym: string; fromOrder: boolean
}) {
  const [q, setQ] = useState('')
  const [newName, setNewName] = useState<string | null>(null)
  const qc = useQueryClient()
  const hits = useQuery({ ...searchQuery(q), placeholderData: keepPreviousData }).data?.items.filter((p) => !p.isService) ?? []
  const set = (i: number, patch: Partial<StockLine>) => onChange(lines.map((l, j) => (j === i ? { ...l, ...patch } : l)))
  const add = (p: TillProduct, cost?: string) => {
    const at = lines.findIndex((l) => l.productId === p.id)
    if (at >= 0) set(at, { quantity: String(num(lines[at].quantity) + 1) })
    else onChange([...lines, blank(p, withPrices, cost)])
    setQ('')
  }
  const addTop = async () => {
    const fresh = (await qc.query(searchQuery(q))).items.filter((p) => !p.isService) // a scan's Enter beats the list
    if (fresh[0]) add(fresh[0])
    else { setNewName(q.trim()); setQ('') } // close the results: they'd cover the new-product row
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col px-5">
      <div className="relative mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input autoFocus aria-label="Add a product" placeholder="Scan or type to add a product" autoComplete="off" value={q}
          onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && q.trim() && void addTop()} className={cn(FIELD, 'w-full pl-9')} />
        {q.trim() && (
          <ul className="absolute inset-x-0 top-full z-10 mt-1 max-h-60 overflow-auto rounded-[var(--radius-sm)] border border-border bg-card p-1 shadow-[var(--shadow)]">
            {hits.slice(0, 8).map((p) => (
              <li key={p.id}>
                <button type="button" onClick={() => add(p)} className="flex w-full justify-between gap-3 rounded-md px-2.5 py-1.5 text-left text-[13px] hover:bg-muted">
                  <span className="truncate">{p.name}</span><small className="text-muted-foreground tabular-nums">{qtyText(p.stockQty)} {p.unitAbbr}</small>
                </button>
              </li>
            ))}
            <li>
              <button type="button" onClick={() => { setNewName(q.trim()); setQ('') }} className="flex w-full items-center gap-1.5 rounded-md px-2.5 py-1.5 text-left text-[13px] font-semibold text-brand hover:bg-muted">
                <Plus size={14} /> New product “{q.trim()}”
              </button>
            </li>
          </ul>
        )}
      </div>
      {newName !== null && <NewProduct name={newName} wanted={!withPrices} onDone={(p, cost) => { setNewName(null); setQ(''); if (p) add(p, cost) }} />}
      <div className={cn('grid items-center gap-1.5 px-1 pb-1 text-[11.5px] font-semibold text-muted-foreground',
        withPrices ? 'grid-cols-[minmax(0,1fr)_64px_52px_76px_76px_24px]' : 'grid-cols-[minmax(0,1fr)_80px_24px]')}>
        <span>Item</span><span className="text-right">Qty</span>
        {withPrices && <><span className="text-right">Free</span><span className="text-right">Cost</span><span className="text-right">Sell</span></>}
        <span />
      </div>
      <ul aria-label="Lines" className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto pb-2">
        {lines.length === 0 && <li className="m-auto p-6 text-center text-faint">{fromOrder ? 'Loading the order…' : 'Nothing added yet.'}</li>}
        {lines.map((l, i) => (
          <li key={l.productId} className="rounded-[var(--radius-sm)] bg-muted px-2 py-1.5">
           <div className={cn('grid items-center gap-1.5', withPrices ? 'grid-cols-[minmax(0,1fr)_64px_52px_76px_76px_24px]' : 'grid-cols-[minmax(0,1fr)_80px_24px]')}>
            <span className="min-w-0">
              <b className="block truncate text-[13px]">{l.name}</b>
              {l.ordered && <small className="text-muted-foreground tabular-nums">ordered {qtyText(l.ordered)}</small>}
            </span>
            <input aria-label={`Quantity of ${l.name}`} inputMode="decimal" value={l.quantity} onChange={(e) => set(i, { quantity: dec(e.target.value) })} className={CELL} />
            {withPrices && (
              <>
                <input aria-label={`Free ${l.name}`} inputMode="decimal" placeholder="0" value={l.freeQty} onChange={(e) => set(i, { freeQty: dec(e.target.value) })} className={CELL} />
                <input aria-label={`Cost of ${l.name}`} inputMode="decimal" placeholder="cost" value={l.costPrice} onChange={(e) => set(i, { costPrice: dec(e.target.value) })}
                  className={cn(CELL, !(num(l.costPrice) > 0) && 'border-warning')} />
                <input aria-label={`Selling price of ${l.name}`} inputMode="decimal" placeholder="same" value={l.sellingPrice} onChange={(e) => set(i, { sellingPrice: dec(e.target.value) })} className={CELL} />
              </>
            )}
            <button type="button" aria-label={`Remove ${l.name}`} onClick={() => onChange(lines.filter((_, j) => j !== i))} className="text-faint hover:text-danger"><X size={15} /></button>
           </div>
            {withPrices && (
              <div className="mt-1 flex flex-wrap items-center gap-1.5 text-[12px] text-muted-foreground">
                Discount
                <input aria-label={`Discount on ${l.name}`} inputMode="decimal" placeholder="0" value={l.discount} onChange={(e) => set(i, { discount: dec(e.target.value) })} className={cn(CELL, 'h-7 w-16')} />
                <Seg label={`Discount type for ${l.name}`} value={l.discountType === 'percent' ? 'percent' : 'fixed'} onChange={(t) => set(i, { discountType: t })}
                  options={[['fixed', `${sym}/unit`], ['percent', '%']]} />
                <span className="ml-2">Expires</span>
                <input type="date" aria-label={`Expiry of ${l.name}`} value={l.expiryDate} onChange={(e) => set(i, { expiryDate: e.target.value })} className={cn(FIELD, 'h-7 w-[140px] px-2 text-[12px]')} />
                {lineGross(l) > 0 && (
                  <b className="ml-auto text-[13px] text-foreground tabular-nums">
                    {lineDiscount(l) > 0 && <s className="mr-1.5 font-normal text-muted-foreground">{money(sym, lineGross(l))}</s>}
                    {money(sym, lineNet(l))}
                  </b>
                )}
              </div>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

function NewProduct({ name, wanted, onDone }: { name: string; wanted: boolean; onDone: (p: TillProduct | null, cost?: string) => void }) {
  const [f, setF] = useState({ name, barcode: '', cost: '', sell: '' })
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      const body: IntakeProductRequest = { name: f.name, barcode: f.barcode, costPrice: f.cost, sellingPrice: f.sell, unitId: 0, wanted }
      const p = await api<TillProduct>('/till/intake-products', { json: body })
      toast.success(`${p.name} added to the catalogue`)
      onDone(p, wanted ? '' : f.cost)
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="mb-2 flex flex-wrap items-center gap-1.5 rounded-[var(--radius-sm)] border border-brand p-2">
      <input autoFocus aria-label="New product name" value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} className={cn(FIELD, 'h-9 min-w-[160px] flex-1')} />
      <input aria-label="New product barcode" placeholder="Barcode" value={f.barcode} onChange={(e) => setF({ ...f, barcode: e.target.value })} className={cn(FIELD, 'h-9 w-32')} />
      {!wanted && (
        <>
          <input aria-label="New product cost" inputMode="decimal" placeholder="Cost" value={f.cost} onChange={(e) => setF({ ...f, cost: e.target.value })} className={cn(FIELD, 'h-9 w-20')} />
          <input aria-label="New product selling price" inputMode="decimal" placeholder="Sell" value={f.sell} onChange={(e) => setF({ ...f, sell: e.target.value })} className={cn(FIELD, 'h-9 w-20')} />
        </>
      )}
      <Button size="sm" disabled={busy} onClick={() => void save()}>Add</Button>
      <Button size="sm" variant="ghost" onClick={() => onDone(null)}>Cancel</Button>
    </div>
  )
}
