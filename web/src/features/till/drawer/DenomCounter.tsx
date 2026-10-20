import type { Denomination } from '@/api/types.gen'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { FIELD } from '../ui'
import { countTotal, type Counts } from './count'

export function DenomCounter({ denoms, counts, onChange, sym, label = 'Counted' }: {
  denoms: Denomination[]
  counts: Counts
  onChange: (c: Counts) => void
  sym: string
  label?: string
}) {
  return (
    <div>
      <div className="grid gap-x-5 gap-y-1.5 sm:grid-cols-2">
        {denoms.map((d) => (
          <label key={d.value} className="flex items-center gap-2 text-[13.5px]">
            <span className="w-24 text-right font-semibold tabular-nums">{money(sym, d.value)}</span>
            <span className="text-faint">×</span>
            <input aria-label={`Count of ${money(sym, d.value)}`} inputMode="numeric" autoComplete="off"
              value={counts[d.value] ?? ''} onChange={(e) => onChange({ ...counts, [d.value]: e.target.value.replace(/\D/g, '') })}
              className={cn(FIELD, 'h-9 w-16 px-2 text-center')} />
            <span className="ml-auto text-muted-foreground tabular-nums">{money(sym, num(d.value) * num(counts[d.value]))}</span>
          </label>
        ))}
      </div>
      <div className="mt-3 flex items-baseline justify-between border-t border-border pt-3">
        <span className="font-semibold text-muted-foreground">{label}</span>
        <span className="font-display text-2xl font-bold tabular-nums">{money(sym, countTotal(denoms, counts))}</span>
      </div>
    </div>
  )
}
