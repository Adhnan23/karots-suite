import { useSuspenseQuery } from '@tanstack/react-query'
import type { DetailRow, LabelRequest } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { LabelPreview } from '@/features/counter/labels/LabelPreview'
import { LABEL_SIZES, labelCount, saveSlot, slotDefault, slotOptions, slotValue } from '@/features/counter/labels/slots'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'

/** The label settings carried from item to item. count '' = follow the quantity. */
export type LabelOpts = { on: boolean; top: string; bottom: string; count: string; size: string }
export const initialLabels = (size: string): LabelOpts => ({ on: true, top: slotDefault('top', 'name'), bottom: slotDefault('bottom', 'price'), count: '', size })

/** The sticker run for an item: its first / second text resolved (name, price or a
 * label field), the count following the quantity unless typed. */
export function labelRequest(o: LabelOpts, it: { productId: number; name: string; price: string; barcode: string; qty: string }, sym: string, fields: DetailRow[] = []): LabelRequest {
  const opts = slotOptions(it.name, money(sym, it.price), fields)
  return { productId: it.productId, code: it.barcode, format: 'CODE128', top: slotValue(opts, o.top), bottom: slotValue(opts, o.bottom), topKey: o.top, bottomKey: o.bottom,
    count: o.count ? labelCount(num(o.count)) : labelCount(num(it.qty)), size: o.size }
}

/** Print-on-save, count, size, the first / second text pickers and a true-to-paper preview (karots-pos intake labels). */
export function LabelBlock({ opts, set, name, price, barcode, qty, fields = [] }: {
  opts: LabelOpts
  set: (o: LabelOpts) => void
  name: string
  price: string
  barcode: string
  qty: string
  fields?: DetailRow[]
}) {
  const cfg = useSuspenseQuery(configQuery).data
  const options = slotOptions(name || 'Name', money(cfg.symbol, price), fields)
  const req = labelRequest(opts, { productId: 0, name: name || 'Name', price, barcode, qty }, cfg.symbol, fields)
  const slot = (which: 'top' | 'bottom') => (
    <select aria-label={which === 'top' ? 'First text' : 'Second text'} value={options.some((o) => o.key === opts[which]) ? opts[which] : 'none'}
      onChange={(e) => { set({ ...opts, [which]: e.target.value }); saveSlot(which, e.target.value) }} className={cn(INPUT, 'h-10 appearance-auto')}>
      {options.map((o) => <option key={o.key} value={o.key}>{o.label}</option>)}
    </select>
  )
  return (
    <div className="flex flex-col gap-3 rounded-[var(--radius-sm)] border border-border p-3">
      <label className="flex items-center gap-2 text-sm font-semibold">
        <input type="checkbox" checked={opts.on} onChange={(e) => set({ ...opts, on: e.target.checked })} /> Print labels on save
      </label>
      {opts.on && (
        <>
          <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
            <label className="flex flex-col gap-1 text-[12px] font-semibold text-muted-foreground">Labels
              <input aria-label="Label count" inputMode="numeric" value={opts.count} placeholder={String(labelCount(num(qty)))} onChange={(e) => set({ ...opts, count: e.target.value.replace(/\D/g, '') })} className={cn(INPUT, 'h-10 text-right tabular-nums')} />
            </label>
            <label className="flex flex-col gap-1 text-[12px] font-semibold text-muted-foreground">Size
              <select aria-label="Label size" value={opts.size} onChange={(e) => set({ ...opts, size: e.target.value })} className={cn(INPUT, 'h-10 appearance-auto')}>
                {[...new Set([cfg.labelSize, ...LABEL_SIZES])].filter(Boolean).map((s) => <option key={s} value={s}>{s.replace('x', ' × ')} mm</option>)}
              </select>
            </label>
            <label className="flex flex-col gap-1 text-[12px] font-semibold text-muted-foreground">First text{slot('top')}</label>
            <label className="flex flex-col gap-1 text-[12px] font-semibold text-muted-foreground">Second text{slot('bottom')}</label>
          </div>
          {barcode ? <div className="rounded-[var(--radius-sm)] bg-muted py-4"><LabelPreview req={req} /></div>
            : <p className="text-[12px] text-warning">No barcode yet — scan or generate one so the label scans at the till.</p>}
        </>
      )}
    </div>
  )
}
