import type { ReactNode } from 'react'

/** An admin page's title row. Breadcrumb and area tabs come from AdminShell. */
export function PageHead({ title, sub, actions }: { title: string; sub?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
      <div className="min-w-0">
        <h1 className="font-display text-[24px] leading-tight font-bold tracking-tight md:text-[28px]">{title}</h1>
        {sub && <p className="mt-0.5 text-sm text-muted-foreground">{sub}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}

/** A card surface for admin content (large surface: glass allowed). */
export const SURFACE = 'rounded-[var(--radius)] border border-border bg-card shadow-[var(--shadow)] [backdrop-filter:var(--glass)]'

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-1 px-6 py-12 text-center">
      <b className="font-display text-lg">{title}</b>
      {children && <div className="max-w-[46ch] text-sm text-muted-foreground">{children}</div>}
    </div>
  )
}
