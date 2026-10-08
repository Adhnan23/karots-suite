import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowRight, Check } from 'lucide-react'
import { api } from '@/api/client'
import type { SetupStep } from '@/api/types.gen'
import { SURFACE } from '@/components/admin/PageHead'
import { cn } from '@/lib/utils'

const setupQuery = { queryKey: ['admin', 'setup'], queryFn: () => api<SetupStep[]>('/admin/setup') }

/** "Get the shop ready" (karots-pos Setup checklist): the go-live steps from
 * live data, the next one first. Admins only; gone once every step is done. */
export function SetupCard() {
  const steps = useQuery(setupQuery).data
  if (!steps || steps.every((s) => s.done)) return null
  const done = steps.filter((s) => s.done).length
  const next = steps.find((s) => !s.done)!
  return (
    <section aria-label="Get the shop ready" className={cn(SURFACE, 'flex flex-col gap-3 p-4 md:p-5')}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="font-display text-lg font-bold">Get the shop ready</h2>
          <p className="text-[13px] text-muted-foreground">{done} of {steps.length} done · next: {next.label.toLowerCase()} — {next.hint.toLowerCase()}</p>
        </div>
        <Link to={next.to} className="inline-flex h-10 items-center gap-1.5 rounded-full bg-primary px-4 text-sm font-bold text-primary-foreground">
          {next.label} <ArrowRight size={16} />
        </Link>
      </div>
      <div aria-hidden className="h-1.5 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-success" style={{ width: `${(done / steps.length) * 100}%` }} />
      </div>
      <ol className="flex flex-wrap gap-1.5">
        {steps.map((s) => (
          <li key={s.key}>
            <Link to={s.to} className={cn('inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-[13px] font-semibold',
              s.done ? 'border-transparent bg-success/12 text-success' : s === next ? 'border-selected bg-selected text-selected-foreground' : 'border-border bg-card text-muted-foreground')}>
              {s.done && <Check size={14} />}{s.label}
            </Link>
          </li>
        ))}
      </ol>
    </section>
  )
}
