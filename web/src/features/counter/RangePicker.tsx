import { FIELD } from '@/features/till/ui'
import { cn } from '@/lib/utils'
import type { Range } from './range'

const PRESETS: [string, string][] = [
  ['', 'Latest'], ['today', 'Today'], ['yesterday', 'Yesterday'], ['this-week', 'This week'], ['last-week', 'Last week'],
  ['this-month', 'This month'], ['last-month', 'Last month'], ['this-year', 'This year'], ['custom', 'Pick dates…'],
]
const SMALL = cn(FIELD, 'h-8 px-2 text-[13px]')

/** When: a preset, or two dates. Sits in a list's header. */
export function RangePicker({ value, onChange }: { value: Range; onChange: (r: Range) => void }) {
  return (
    <span className="flex flex-wrap items-center justify-end gap-1.5">
      <select aria-label="Period" value={value.preset} onChange={(e) => onChange({ ...value, preset: e.target.value })} className={cn(SMALL, 'appearance-auto')}>
        {PRESETS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
      </select>
      {value.preset === 'custom' && (
        <>
          <input type="date" aria-label="From" value={value.from} onChange={(e) => onChange({ ...value, from: e.target.value })} className={cn(SMALL, 'w-[140px]')} />
          <span className="text-muted-foreground">–</span>
          <input type="date" aria-label="To" value={value.to} min={value.from} onChange={(e) => onChange({ ...value, to: e.target.value })} className={cn(SMALL, 'w-[140px]')} />
        </>
      )}
    </span>
  )
}
