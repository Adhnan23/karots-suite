import { useState } from 'react'
import type { AdminPack } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { Seg } from '@/features/till/bits'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'

/** Restock counted in cards (or boxes): which way the count is, the card size to
 * check against the delivery, and the invoice's cost per card → cost per unit. */
export function PackEntry({ packs, pack, unit, cost, sym, onPick, onPackCost }: {
  packs: AdminPack[]; pack?: AdminPack; unit: string; cost: string; sym: string
  onPick: (id: number) => void; onPackCost: (unitCost: string) => void
}) {
  const size = pack ? num(pack.size) : 1
  const [packCost, setPackCost] = useState('')
  const opts: [string, string][] = [['0', unit], ...packs.slice(0, 3).map((k): [string, string] => [String(k.id), `${k.name} ×${qtyText(k.size)}`])]
  return (
    <div className="flex flex-col gap-2 rounded-[var(--radius-sm)] bg-muted p-3 text-[13px]">
      <span className="flex flex-wrap items-center gap-2">
        Counting in <Seg label="Count in" value={String(pack?.id ?? 0)} options={opts} onChange={(v) => { setPackCost(''); onPick(Number(v)) }} />
      </span>
      {pack && (
        <>
          <span className="font-semibold">Check: does one {pack.name.toLowerCase()} still hold {qtyText(pack.size)} {unit}? If not, change it in the product form.</span>
          <label className="flex flex-wrap items-center gap-2">
            Cost per {pack.name.toLowerCase()}
            <input aria-label={`Cost per ${pack.name}`} inputMode="decimal" value={packCost} placeholder={(num(cost) * size).toFixed(2)}
              onChange={(e) => { const v = e.target.value.replace(/[^\d.]/g, ''); setPackCost(v); if (num(v) > 0) onPackCost((num(v) / size).toFixed(2)) }}
              className={cn(INPUT, 'h-9 w-28 tabular-nums')} />
            <span className="text-muted-foreground">= {money(sym, cost)} a {unit}</span>
          </label>
        </>
      )}
    </div>
  )
}
