import type { Locker } from '@/api/types.gen'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { FIELD } from '../ui'

/** Where the cash comes from / goes to. untrackedLabel omitted = untracked not allowed. */
export function LockerSelect({ label, lockers, value, onChange, untrackedLabel, sym }: {
  label: string
  lockers: Locker[]
  value: string
  onChange: (v: string) => void
  untrackedLabel?: string
  sym: string
}) {
  if (!lockers.length) return null
  return (
    <label className="flex flex-col gap-1 text-[13px] font-semibold text-muted-foreground">
      {label}
      <select value={value} onChange={(e) => onChange(e.target.value)} className={cn(FIELD, 'appearance-auto')}>
        {untrackedLabel && <option value="">{untrackedLabel}</option>}
        {lockers.map((l) => (
          <option key={l.id} value={String(l.id)}>{l.name} ({money(sym, l.balance)})</option>
        ))}
      </select>
    </label>
  )
}
