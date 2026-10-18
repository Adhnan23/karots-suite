import { CHIP, FIELD } from '@/features/till/ui'
import { cn } from '@/lib/utils'
import { money, num } from '@/lib/money'
import { LABEL } from './words'


/** An amount box; digits and one dot only. */
export function Amount({ label, value, onChange, big, placeholder }: { label: string; value: string; onChange: (v: string) => void; big?: boolean; placeholder?: string }) {
  return (
    <label className={LABEL}>
      {label}
      <input aria-label={label} inputMode="decimal" autoComplete="off" placeholder={placeholder ?? '0.00'} value={value}
        onChange={(e) => onChange(e.target.value.replace(/[^\d.]/g, ''))}
        className={cn(FIELD, 'w-full min-w-0 tabular-nums', big && 'h-14 font-display text-2xl font-bold')} />
    </label>
  )
}

/** Pick one of the shop's floats (or a bank): chips with the balance, red below zero. */
export function Pick<T extends { key: string; label: string; balance?: string }>({ label, items, value, onChange, sym }: {
  label: string; items: T[]; value: string; onChange: (k: string) => void; sym: string
}) {
  return (
    <div role="group" aria-label={label} className="flex flex-wrap gap-1.5">
      {items.map((it) => (
        <button key={it.key} type="button" className={CHIP} aria-pressed={value === it.key} onClick={() => onChange(it.key)}>
          {it.label}
          {it.balance !== undefined && (
            <span className={cn('tabular-nums font-normal', num(it.balance) < 0 ? 'text-danger' : 'opacity-70')}>{money(sym, it.balance)}</span>
          )}
        </button>
      ))}
    </div>
  )
}

/** A word when the float would go below zero — the sale still goes ahead. */
export function BelowZero({ show }: { show: boolean }) {
  if (!show) return null
  return <p role="status" className="text-[12.5px] font-semibold text-warning">This takes the float below zero — it still sells; count the phone after.</p>
}
