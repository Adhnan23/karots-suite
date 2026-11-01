import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

/** The phone screens' small shared pieces (mockup: docs/design/mockups/pocket-cashier.html). */

export const GLASS = '[-webkit-backdrop-filter:var(--glass)] [backdrop-filter:var(--glass)]'
export const CARD = cn('rounded-[var(--radius-sm)] border border-border bg-card shadow-sm', GLASS)

export function LivePill({ live, label }: { live: boolean; label?: string }) {
  return (
    <span role="status" className={cn('inline-flex items-center gap-1.5 whitespace-nowrap rounded-pill px-2.5 py-1 text-[11.5px] font-semibold',
      live ? 'bg-[color-mix(in_srgb,var(--success)_16%,transparent)] text-success' : 'bg-[color-mix(in_srgb,var(--warning)_18%,transparent)] text-warning')}>
      <span className="size-[7px] rounded-full bg-current" />
      {label ?? (live ? 'Live' : 'PC not reachable')}
    </span>
  )
}

export function Pill({ children }: { children: ReactNode }) {
  return <span className="inline-flex items-center gap-1.5 whitespace-nowrap rounded-pill bg-muted px-2.5 py-1 text-[11.5px] font-semibold text-muted-foreground">{children}</span>
}

export const initials = (name: string) => name.split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0]?.toUpperCase()).join('') || '?'

/** "a few seconds ago", "5 min ago", "2 h ago". */
export function ago(iso: string | null, now = Date.now()) {
  if (!iso) return 'never'
  const s = Math.max(0, Math.floor((now - Date.parse(iso)) / 1000))
  if (s < 45) return 'a few seconds ago'
  if (s < 3600) return `${Math.max(1, Math.round(s / 60))} min ago`
  return `${Math.floor(s / 3600)} h ago`
}

export const hhmm = (iso: string | null) => (iso ? new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '—')

/** The phone frame: full height, the palette's background art, safe areas.
 * fixed = exactly the screen's height (the signed-in app: bars stay, lists scroll). */
export function Screen({ children, fixed }: { children: ReactNode; fixed?: boolean }) {
  return (
    <div className={cn('flex flex-col bg-background bg-[image:var(--bg-art)] pt-[env(safe-area-inset-top)] pb-[env(safe-area-inset-bottom)] text-foreground',
      fixed ? 'h-dvh overflow-hidden' : 'min-h-dvh')}>
      {children}
    </div>
  )
}
