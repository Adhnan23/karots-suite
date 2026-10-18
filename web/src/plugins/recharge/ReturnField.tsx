import { useQuery } from '@tanstack/react-query'
import type { SaleLine as CoreLine } from '@/api/types.gen'
import { Seg } from '@/features/till/bits'
import { configQuery } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { saleLinesQuery } from './api'

type Back = { back?: string }

/** On a reload / bill being returned: how much came back from the carrier (or the bank).
 * All = it never went through; None = wrong number, the float is gone; or part of it. */
export function ReturnField({ saleId, line, value, onChange }: { saleId: number; line: CoreLine; qty: number; value: unknown; onChange: (v: unknown) => void }) {
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const mine = useQuery(saleLinesQuery(saleId)).data?.find((l) => l.saleItemId === line.id)
  if (!mine) return null
  const back = (value as Back | undefined)?.back ?? 'all'
  const mode = back === 'all' || back === 'none' ? back : 'part'
  const what = mine.kind === 'bank' ? 'the bank' : 'the float'
  return (
    <div className="mt-1.5 flex flex-wrap items-center gap-2 text-[12.5px] text-muted-foreground">
      <span>Back to {what} ({mine.label})?</span>
      <Seg label={`Came back to ${what}`} value={mode} onChange={(m) => onChange({ back: m === 'part' ? '' : m })}
        options={[['all', `All ${money(sym, mine.left)}`], ['none', mine.kind === 'bank' ? 'None' : 'None — wrong number'], ['part', 'Part']]} />
      {mode === 'part' && (
        <input aria-label="Amount that came back" inputMode="decimal" autoComplete="off" placeholder="0.00" value={back}
          onChange={(e) => onChange({ back: e.target.value.replace(/[^\d.]/g, '') })} className={cn(FIELD, 'h-8 w-24 text-[13px] tabular-nums')} />
      )}
      {mode === 'part' && num(back) > num(mine.left) && <span className="text-warning">at most {money(sym, mine.left)}</span>}
    </div>
  )
}
