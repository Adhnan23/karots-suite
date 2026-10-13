import { useState } from 'react'
import type { LotRow, MergeBatchesRequest } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { mergeDefaults } from './batches'

const CELL = cn(INPUT, 'h-10 text-right text-sm font-normal text-foreground tabular-nums')
const fmtDay = (ymd: string) => new Date(`${ymd}T00:00:00`).toLocaleDateString([], { day: 'numeric', month: 'short', year: 'numeric' })

/** The merge prompt: the ticked batches fold into the oldest. Every value is
 * pre-filled (weighted cost, earliest expiry, the oldest's price) and can be changed. */
export function MergePrompt({ lots, unit, sym, productPrice, busy, onMerge, onCancel }: {
  lots: LotRow[]; unit: string; sym: string; productPrice: string; busy: boolean
  onMerge: (r: MergeBatchesRequest) => void; onCancel: () => void
}) {
  const d = mergeDefaults(lots)
  const [cost, setCost] = useState(d.cost)
  const [expiry, setExpiry] = useState(d.expiry)
  const [price, setPrice] = useState(d.price)
  const valueMove = num(cost) * d.qty - d.value
  const prices = [...new Set(lots.map((l) => l.sellingPrice))]
  return (
    <div role="group" aria-label="Merge batches" className="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-brand bg-brand-soft p-2.5 text-sm">
      <b>Merge {lots.length} batches into one of {qtyText(d.qty)} {unit}</b>
      <div className="grid grid-cols-2 gap-2">
        <label className="flex flex-col gap-0.5 text-[11px] font-semibold text-muted-foreground">
          Cost
          <input aria-label="Merged cost" inputMode="decimal" value={cost} onChange={(e) => setCost(e.target.value.replace(/[^\d.]/g, ''))} className={CELL} />
        </label>
        <label className="flex flex-col gap-0.5 text-[11px] font-semibold text-muted-foreground">
          Expiry
          <input aria-label="Merged expiry" type="date" value={expiry} onChange={(e) => setExpiry(e.target.value)} className={cn(INPUT, 'h-10 text-sm font-normal text-foreground')} />
        </label>
      </div>
      {Math.abs(valueMove) >= 0.01 && (
        <small className="text-[12px] text-warning">Stock value {valueMove > 0 ? '+' : '−'}{money(sym, Math.abs(valueMove))} (average was {money(sym, d.cost)})</small>
      )}
      {d.expiriesDiffer && expiry && (
        <small className="text-[12px] text-warning">The later ones will be sold as if they expire {fmtDay(expiry)}.</small>
      )}
      {d.pricesDiffer && (
        <div className="flex flex-col gap-1">
          <span className="text-[11px] font-semibold text-muted-foreground">Sell at</span>
          <div className="flex flex-wrap gap-1.5">
            {prices.map((p) => (
              <Button key={p || 'follow'} type="button" size="sm" variant={price === p ? 'default' : 'outline'} aria-pressed={price === p} onClick={() => setPrice(p)}>
                {p ? money(sym, p) : `Follow the product (${money(sym, productPrice)})`}
              </Button>
            ))}
          </div>
        </div>
      )}
      <small className="text-[11.5px] text-faint">The count stays the same. The emptied batches stay in the history.</small>
      <div className="flex justify-end gap-1.5">
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>Cancel</Button>
        <Button type="button" size="sm" disabled={busy || cost === ''}
          onClick={() => onMerge({ batchIds: lots.map((l) => l.id), cost, expiry, sellingPrice: price })}>Merge</Button>
      </div>
    </div>
  )
}
