import { useState } from 'react'
import type { AddBatchRequest, BatchCount, LotRow } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { AddBatchForm } from './AddBatchForm'
import { ExpiryTag } from './LotsSheet'

/** A stock-take row counted batch by batch: each batch's count, cost and own
 * selling price, plus batches found that aren't listed. Kept with the row's
 * edits until Save counts. */
export function StockTakeLots({ name, unit, productPrice, wholesale, lots, batches, added, onBatches, onAdded }: {
  name: string; unit: string; productPrice: string; wholesale: string; lots: LotRow[]; batches: BatchCount[]; added: AddBatchRequest[]
  onBatches: (b: BatchCount[]) => void; onAdded: (a: AddBatchRequest[]) => void
}) {
  const [adding, setAdding] = useState(false)
  const line = (id: number): BatchCount => batches.find((b) => b.batchId === id) ?? { batchId: id, counted: '', cost: '', selling: '' }
  const set = (id: number, patch: Partial<BatchCount>) => {
    const next = { ...line(id), ...patch }
    onBatches([...batches.filter((b) => b.batchId !== id), ...(next.counted || next.cost || next.selling ? [next] : [])])
  }
  const clean = (v: string) => v.replace(/[^\d.]/g, '')
  return (
    <div aria-label={`Batches of ${name}`} role="group" className="col-span-2 flex flex-col gap-4 rounded-[var(--radius-sm)] bg-muted p-2 md:col-span-6 md:-mx-2">
      {lots.map((l, i) => {
        const b = line(l.id)
        const v = b.counted
        const d = v === '' ? 0 : num(v) - num(l.qtyRemaining)
        return (
          <div key={l.id} className="grid grid-cols-[minmax(0,1fr)_110px] items-center gap-2 md:grid-cols-[minmax(0,2fr)_90px_110px_110px_110px_110px]">
            <span className="min-w-0 text-sm md:col-span-2">
              <b className="tabular-nums">{qtyText(l.qtyRemaining)}</b> <small className="text-faint">{unit}</small> <ExpiryTag ymd={l.expiryDate} />
              <small className="block text-faint">Batch {i + 1}{l.expiryDate && ` · exp ${l.expiryDate}`}{!l.sellingPrice && ' · follows the product price'}</small>
            </span>
            <label className="relative">
              <input aria-label={`Counted ${name} batch ${i + 1}`} inputMode="decimal" value={v} placeholder={qtyText(l.qtyRemaining)}
                onChange={(e) => set(l.id, { counted: clean(e.target.value) })} className={cn(INPUT, 'h-10 px-2.5 text-right font-bold tabular-nums')} />
              {d !== 0 && <small className={cn('absolute -bottom-3.5 right-1 text-[10.5px] font-bold', d > 0 ? 'text-success' : 'text-danger')}>{d > 0 ? '+' : '−'}{qtyText(Math.abs(d))}</small>}
            </label>
            <div className="col-span-2 grid grid-cols-3 gap-2 md:contents">
              {([['cost', 'Cost', l.costPrice], ['selling', 'Selling', l.sellingPrice || productPrice]] as const).map(([k, lb, ph]) => (
                <label key={k} className="flex flex-col gap-0.5">
                  <small className="text-[11px] font-semibold text-faint md:hidden">{lb}</small>
                  <input aria-label={`${lb} of ${name} batch ${i + 1}`} inputMode="decimal" value={b[k]} placeholder={ph}
                    onChange={(e) => set(l.id, { [k]: clean(e.target.value) })} className={cn(INPUT, 'h-10 px-2.5 text-right tabular-nums')} />
                </label>
              ))}
              {/* Wholesale is the product's, the same for every batch: edit it on the product row above. */}
              <label className="flex flex-col gap-0.5" title="Wholesale is the product's — the same for every batch">
                <small className="text-[11px] font-semibold text-faint md:hidden">Wholesale</small>
                <input aria-label={`Wholesale of ${name} batch ${i + 1}`} disabled value="" placeholder={num(wholesale) ? wholesale : '—'}
                  className={cn(INPUT, 'h-10 px-2.5 text-right tabular-nums opacity-60')} />
              </label>
            </div>
          </div>
        )
      })}
      {num(wholesale) > 0 && <small className="text-[11.5px] text-faint">Wholesale is the product’s — the same for every batch. Change it on the row above.</small>}
      {added.map((a, i) => (
        <div key={i} className="flex items-center gap-2 text-sm">
          <span className="flex-1">New batch: <b className="tabular-nums">{qtyText(a.qty)}</b> {unit}{a.cost && ` · cost ${a.cost}`}{a.expiry && ` · exp ${a.expiry}`}</span>
          <Button type="button" variant="ghost" size="sm" aria-label={`Drop new batch ${i + 1}`} onClick={() => onAdded(added.filter((_, j) => j !== i))}>×</Button>
        </div>
      ))}
      {adding ? (
        <AddBatchForm cost="product's" price="follows" busy={false} onCancel={() => setAdding(false)} onSave={(b) => { onAdded([...added, b]); setAdding(false) }} />
      ) : (
        <Button type="button" variant="outline" size="sm" className="self-start" onClick={() => setAdding(true)}>+ batch</Button>
      )}
    </div>
  )
}
