import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

/** Admin text input / select look (same family as the till FIELD). */
export const INPUT =
  'h-[var(--control-h)] w-full rounded-[var(--radius-sm)] border border-border bg-muted px-3 text-[15px] text-foreground outline-none placeholder:text-faint focus-visible:ring-2 focus-visible:ring-ring aria-invalid:border-danger'

export function Field({ label, hint, children, className }: { label: string; hint?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <label className={cn('flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground', className)}>
      {label}
      {children}
      {hint && <small className="font-normal text-faint">{hint}</small>}
    </label>
  )
}

/** A labelled on/off switch row. */
export function Toggle({ label, hint, checked, onChange }: { label: string; hint?: ReactNode; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-2.5">
      <span className="flex flex-col text-sm font-semibold">
        {label}
        {hint && <small className="font-normal text-faint">{hint}</small>}
      </span>
      <input type="checkbox" role="switch" checked={checked} onChange={(e) => onChange(e.target.checked)}
        className="peer sr-only" />
      <span aria-hidden className="relative h-6 w-10 flex-none rounded-full bg-border transition-colors peer-checked:bg-selected peer-focus-visible:ring-2 peer-focus-visible:ring-ring after:absolute after:top-0.5 after:left-0.5 after:h-5 after:w-5 after:rounded-full after:bg-card after:shadow after:transition-transform peer-checked:after:translate-x-4" />
    </label>
  )
}
