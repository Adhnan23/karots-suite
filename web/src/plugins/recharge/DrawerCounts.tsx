import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { configQuery, errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { drawerCounts, tillDataQuery } from './api'

/** An optional float check in the till's Close dialog: folded away until tapped (the
 * commission is booked with each move, so the float is right without it). Opened, each
 * device shows its expected float; type what the phone shows (blank = not counted).
 * The difference is booked — a promo, a rate that changed, or money short. */
export function DrawerCounts({ register }: { which: 'open' | 'close'; register: (save: () => Promise<boolean>) => void }) {
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const floats = useQuery(tillDataQuery).data?.floats ?? []
  const [counted, setCounted] = useState<Record<number, string>>({})
  const [open, setOpen] = useState(false)
  register(async () => {
    const counts = floats.filter((f) => (counted[f.id] ?? '') !== '').map((f) => ({ deviceId: f.id, counted: counted[f.id] }))
    if (!counts.length) return true
    try {
      const res = await drawerCounts(counts)
      const off = res.filter((r) => Math.abs(num(r.diff)) > 0.005)
      if (off.length) toast(`Float counted — ${off.map((r) => `${num(r.diff) > 0 ? '+' : ''}${money(sym, r.diff)}`).join(', ')} booked`)
    } catch (e) {
      toast.error(errMsg(e, 'The float count')) // never holds the drawer up
    }
    return true
  })
  if (!floats.length) return null
  if (!open) {
    return (
      <button type="button" onClick={() => setOpen(true)}
        className="flex flex-col items-start gap-0.5 rounded-[var(--radius-sm)] border border-dashed border-border px-3 py-2.5 text-left hover:bg-muted">
        <b className="text-[13.5px]">📲 Check the reload phones</b>
        <small className="text-muted-foreground">Optional — type what each phone shows</small>
      </button>
    )
  }
  return (
    <section aria-label="Reload float" className="flex flex-col gap-2">
      <h3 className="text-[14px] font-bold">Reload float <small className="font-normal text-muted-foreground">— what each phone shows (blank = skip)</small></h3>
      <ul className="flex flex-col gap-1.5">
        {floats.map((f) => {
          const v = counted[f.id] ?? ''
          const diff = v === '' ? 0 : num(v) - num(f.balance)
          return (
            <li key={f.id} className="grid grid-cols-[minmax(0,1fr)_120px] items-center gap-2">
              <span className="min-w-0">
                <b className="block truncate text-[13.5px]">{f.carrier} · {f.label}</b>
                <small className="tabular-nums text-muted-foreground">
                  expected {money(sym, f.balance)}
                  {v !== '' && Math.abs(diff) > 0.005 && <span className={diff > 0 ? 'text-success' : 'text-danger'}> · {diff > 0 ? '+' : ''}{money(sym, diff)}</span>}
                </small>
              </span>
              <input aria-label={`Float on ${f.carrier} ${f.label}`} inputMode="decimal" autoComplete="off" placeholder={num(f.balance).toFixed(2)} value={v}
                onChange={(e) => setCounted((c) => ({ ...c, [f.id]: e.target.value.replace(/[^\d.]/g, '') }))} className={cn(FIELD, 'text-right tabular-nums')} />
            </li>
          )
        })}
      </ul>
    </section>
  )
}
