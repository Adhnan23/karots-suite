import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

/** A figure with a bar behind it sized against the biggest row (karots-pos
 * barWidth: rows come biggest first, so row 0 sets the scale). */
export function BarCell({ pct, children, tone = 'brand' }: { pct: number | null; children: ReactNode; tone?: 'brand' | 'success' | 'danger' | 'warning' }) {
  const w = pct && Number.isFinite(pct) && pct > 0 ? Math.min(100, pct) : 0
  return (
    <span className="relative flex min-w-[96px] items-center justify-end">
      <span aria-hidden className={cn('absolute inset-y-0.5 left-0 rounded-[4px] opacity-20',
        { brand: 'bg-brand', success: 'bg-success', danger: 'bg-danger', warning: 'bg-warning' }[tone])} style={{ width: `${w}%` }} />
      <span className="relative tabular-nums">{children}</span>
    </span>
  )
}
