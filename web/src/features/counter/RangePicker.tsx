import { FIELD } from '@/features/till/ui'
import { cn } from '@/lib/utils'
import type { Range } from './range'

const PRESETS: [string, string][] = [
  ['', 'Latest'], ['today', 'Today'], ['yesterday', 'Yesterday'], ['this-week', 'This week'], ['last-week', 'Last week'],
  ['this-month', 'This month'], ['last-month', 'Last month'], ['this-year', 'This year'], ['custom', 'Pick dates…'],
]
const SMALL = cn(FIELD, 'h-8 px-2 text-[13px]')

/** When: a preset, or two dates. Sits in a list's header; `full` = admin filter-bar size. */
export function RangePicker({ value, onChange, full }: { value: Range; onChange: (r: Range) => void; full?: boolean }) {
  const size = full ? cn(FIELD, 'w-full md:w-auto') : SMALL
  return (
    <span className={cn('flex flex-wrap items-center gap-1.5', !full && 'justify-end')}>
      <select aria-label="Period" value={value.preset} onChange={(e) => onChange({ ...value, preset: e.target.value })} className={cn(size, 'appearance-auto')}>
        {PRESETS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
      </select>
      {value.preset === 'custom' && (
        <>
          <input type="date" aria-label="From" value={value.from} onChange={(e) => onChange({ ...value, from: e.target.value })} className={cn(size, 'w-[150px]')} />
          <span className="text-muted-foreground">–</span>
          <input type="date" aria-label="To" value={value.to} min={value.from} onChange={(e) => onChange({ ...value, to: e.target.value })} className={cn(size, 'w-[150px]')} />
        </>
      )}
    </span>
  )
}
