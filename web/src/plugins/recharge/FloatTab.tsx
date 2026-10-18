import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import type { TillData } from './types'

const ago = (iso: string | null) => (iso ? new Date(iso).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' }) : 'never')

/** 💰 Float: every device's float now (red below zero). */
export function FloatTab({ data, sym }: { data: TillData; sym: string }) {
  return (
    <div className="flex flex-col gap-4">
      <ul aria-label="Floats" className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,300px),1fr))] gap-1.5">
        {data.floats.map((f) => (
          <li key={f.id} className="flex items-center justify-between gap-3 rounded-[var(--radius-sm)] bg-muted px-3 py-2">
            <span className="min-w-0">
              <b className="block truncate text-[14px]">{f.carrier} · {f.label}</b>
              <small className="text-muted-foreground">counted {ago(f.lastCount)}</small>
            </span>
            <b className={cn('shrink-0 whitespace-nowrap font-display text-lg tabular-nums', num(f.balance) < 0 && 'text-danger')}>{money(sym, f.balance)}</b>
          </li>
        ))}
      </ul>
      <p className="text-[12.5px] text-muted-foreground">Buying float from the carrier is on the Suppliers tab.</p>
    </div>
  )
}
