import { useState } from 'react'
import type { AddBatchRequest } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const CELL = cn(INPUT, 'h-10 text-right text-sm tabular-nums')
const clean = (v: string) => v.replace(/[^\d.]/g, '')

/** A batch typed in by hand: qty, cost, its own price (blank = follows the
 * product) and an expiry. */
export function AddBatchForm({ cost, price, busy, onSave, onCancel }: {
  cost: string; price: string; busy: boolean; onSave: (b: AddBatchRequest) => void; onCancel: () => void
}) {
  const [b, setB] = useState<AddBatchRequest>({ qty: '', cost: '', sellingPrice: '', expiry: '' })
  const set = (k: keyof AddBatchRequest, v: string) => setB((x) => ({ ...x, [k]: v }))
  return (
    <form aria-label="New batch" className="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-brand bg-brand-soft p-2.5"
      onSubmit={(e) => { e.preventDefault(); onSave(b) }}>
      <b className="text-sm">New batch</b>
      <div className="grid grid-cols-2 gap-2">
        {([['qty', 'Quantity', ''], ['cost', 'Cost', cost], ['sellingPrice', 'Batch price', price]] as const).map(([k, l, ph]) => (
          <label key={k} className="flex flex-col gap-0.5 text-[11px] font-semibold text-muted-foreground">
            {l}
            <input aria-label={l} autoFocus={k === 'qty'} inputMode="decimal" value={b[k]} placeholder={ph} onChange={(e) => set(k, clean(e.target.value))} className={cn(CELL, 'font-normal text-foreground')} />
          </label>
        ))}
        <label className="flex flex-col gap-0.5 text-[11px] font-semibold text-muted-foreground">
          Expiry
          <input aria-label="Expiry" type="date" value={b.expiry} onChange={(e) => set('expiry', e.target.value)} className={cn(INPUT, 'h-10 text-sm font-normal text-foreground')} />
        </label>
      </div>
      <small className="text-[11.5px] text-faint">Blank cost = the product's cost. Blank batch price = follows the product.</small>
      <div className="flex justify-end gap-1.5">
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>Cancel</Button>
        <Button type="submit" size="sm" disabled={busy || !(Number(b.qty) > 0)}>Add batch</Button>
      </div>
    </form>
  )
}
