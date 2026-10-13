import type { LotRow } from '@/api/types.gen'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'

/** Where a restock goes: one of the product's batches (topped up, keeps its own
 * cost / price / expiry) or a new batch. auto = the default picked by matchBatch. */
export function BatchChoice({ lots, value, auto, unit, sym, productPrice, onPick }: {
  lots: LotRow[]; value: number; auto: number; unit: string; sym: string; productPrice: string; onPick: (id: number) => void
}) {
  const opt = (id: number, title: string, sub: string) => (
    <label key={id} className={cn('flex cursor-pointer items-center gap-2.5 rounded-[var(--radius-sm)] border border-border px-3 py-2', value === id && 'border-brand bg-brand-soft')}>
      <input type="radio" name="in-batch" checked={value === id} onChange={() => onPick(id)} className="size-4 accent-[var(--brand)]" />
      <span className="min-w-0 flex-1 text-sm">
        <b>{title}</b>{id === auto && <small className="ml-1.5 font-semibold text-brand">{id ? 'matches' : 'nothing matches'}</small>}
        <small className="block text-faint">{sub}</small>
      </span>
    </label>
  )
  return (
    <fieldset className="flex flex-col gap-1.5">
      <legend className="mb-1 text-[13px] font-semibold text-muted-foreground">Add it to</legend>
      {lots.map((l, i) => opt(l.id, `Batch ${i + 1} · ${qtyText(l.qtyRemaining)} ${unit}`,
        `cost ${money(sym, l.costPrice)} · sells at ${money(sym, l.sellingPrice || productPrice)}${l.expiryDate ? ` · exp ${l.expiryDate}` : ''}`))}
      {opt(0, 'New batch', 'with the cost, price and expiry above')}
    </fieldset>
  )
}
