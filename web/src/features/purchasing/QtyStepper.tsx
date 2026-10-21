import { Minus, Plus } from 'lucide-react'
import { num } from '@/lib/money'
import { cn } from '@/lib/utils'

/** − qty + pill (the till cart's and the counter return's stepper) for touch
 * screens; the number stays typeable. Never below 0, nor above max when given. */
export function QtyStepper({ value, onChange, label, name, max, className }: {
  value: string; onChange: (v: string) => void; label: string; name: string; max?: number; className?: string
}) {
  const step = (d: number) => {
    const n = Math.max(0, num(value) + d)
    onChange(String(max === undefined ? n : Math.min(n, max)))
  }
  return (
    <div className={cn('flex h-9 items-center rounded-pill border border-border bg-card p-0.5', className)}>
      <button type="button" aria-label={`Fewer ${name}`} onClick={() => step(-1)} className="grid h-8 w-8 flex-none place-items-center rounded-full text-muted-foreground hover:bg-muted"><Minus size={14} /></button>
      <input aria-label={label} inputMode="decimal" autoComplete="off" placeholder="0" value={value} onChange={(e) => onChange(e.target.value.replace(/[^\d.]/g, ''))}
        className="w-full min-w-0 bg-transparent text-center text-[14px] font-bold tabular-nums outline-none" />
      <button type="button" aria-label={`More ${name}`} onClick={() => step(1)} className="grid h-8 w-8 flex-none place-items-center rounded-full text-muted-foreground hover:bg-muted"><Plus size={14} /></button>
    </div>
  )
}
