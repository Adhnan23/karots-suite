import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export function Kbd({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <kbd className={cn('rounded-md border border-border bg-muted px-1.5 py-0.5 font-sans text-[11px] font-semibold text-muted-foreground', className)}>
      {children}
    </kbd>
  )
}

/** Two-to-four option segmented control (Retail/Wholesale, Rs/%). */
export function Seg<T extends string>({ label, value, options, onChange }: {
  label: string
  value: T
  options: [T, string][]
  onChange: (v: T) => void
}) {
  return (
    <div role="group" aria-label={label} className="inline-flex rounded-full border border-border bg-muted p-0.5">
      {options.map(([v, text]) => (
        <button key={v} type="button" aria-pressed={value === v} onClick={() => onChange(v)}
          className="rounded-full px-2.5 py-1 text-[12px] font-bold text-muted-foreground aria-pressed:bg-selected aria-pressed:text-selected-foreground">
          {text}
        </button>
      ))}
    </div>
  )
}
