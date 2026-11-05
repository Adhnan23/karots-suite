import type { PackRequest } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { num } from '@/lib/money'
import { cn } from '@/lib/utils'

const NAMES = ['Card', 'Box', 'Packet', 'Strip', 'Tray', 'Bottle']
export const blankPack: PackRequest = { name: 'Card', size: '', sellingPrice: '', wholesalePrice: '', barcode: '' }

/** One pack's fields, for a saved product (PacksBlock) and a new one (NewPacks). */
export function PackFields({ f, set, unit, unitPrice, err, busy, saveLabel, onCancel, onSave }: {
  f: PackRequest; set: (patch: Partial<PackRequest>) => void; unit: string; unitPrice: string
  err: string; busy: boolean; saveLabel: string; onCancel: () => void; onSave: () => void
}) {
  return (
    <div className="grid gap-2 rounded-[var(--radius-sm)] bg-muted p-2.5" onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); onSave() } }}>
      <div className="flex flex-wrap gap-1.5">
        {NAMES.map((n) => (
          <button key={n} type="button" aria-pressed={f.name === n} onClick={() => set({ name: n })}
            className="rounded-pill border border-border bg-card px-3 py-1 text-[13px] font-semibold aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">{n}</button>
        ))}
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <label className="grid gap-1 text-[12.5px] font-semibold">Name
          <input aria-label="Pack name" value={f.name} onChange={(e) => set({ name: e.target.value })} className={INPUT} />
        </label>
        <label className="grid gap-1 text-[12.5px] font-semibold">{unit} in one
          <input aria-label="Units in one pack" inputMode="numeric" value={f.size} onChange={(e) => set({ size: e.target.value.replace(/[^\d.]/g, '') })} className={cn(INPUT, 'tabular-nums')} placeholder="12" />
        </label>
        <label className="grid gap-1 text-[12.5px] font-semibold">Price
          <input aria-label="Pack price" inputMode="decimal" value={f.sellingPrice} onChange={(e) => set({ sellingPrice: e.target.value.replace(/[^\d.]/g, '') })}
            className={cn(INPUT, 'tabular-nums')} placeholder={num(f.size) > 1 ? (num(f.size) * num(unitPrice)).toFixed(2) : '0.00'} />
        </label>
        <label className="grid gap-1 text-[12.5px] font-semibold">Wholesale
          <input aria-label="Pack wholesale price" inputMode="decimal" value={f.wholesalePrice} onChange={(e) => set({ wholesalePrice: e.target.value.replace(/[^\d.]/g, '') })} className={cn(INPUT, 'tabular-nums')} placeholder="—" />
        </label>
      </div>
      <input aria-label="Pack barcode" name="pack-code" value={f.barcode} onChange={(e) => set({ barcode: e.target.value })}
        placeholder="Scan the code on the pack (optional)" className={cn(INPUT, 'tabular-nums')} />
      <small className="text-faint">A blank price follows {unit} price × size. Full packs always ring at the cheaper of the two.</small>
      {err && <small role="alert" className="text-danger">{err}</small>}
      <div className="flex justify-end gap-1.5">
        <Button type="button" variant="ghost" className="h-9" onClick={onCancel}>Cancel</Button>
        <Button type="button" className="h-9 font-bold" disabled={busy || !(num(f.size) > 1) || !f.name.trim()} onClick={onSave}>
          {saveLabel}
        </Button>
      </div>
    </div>
  )
}
