import { useQuery } from '@tanstack/react-query'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery } from '../api'

/** Top-bar drawer status: "Till open · float Rs … · in drawer Rs …" (≥1440; the amount only below). */
export function DrawerPill() {
  const cfg = useQuery(configQuery).data
  const d = useQuery(drawerQuery).data
  if (!cfg || !d) return null
  const s = d.summary.session
  const full = s ? `Till open · float ${money(cfg.symbol, s.openingCash)} · in drawer ${money(cfg.symbol, d.summary.expected)}` : 'Till closed'
  // Below 1440 the bar also carries the counter tabs: keep only the number that matters.
  return (
    <span title={full} className="flex items-center gap-2 whitespace-nowrap rounded-full border border-border bg-muted px-3 py-1.5 text-[13px]">
      <span className={cn('h-2 w-2 rounded-full', s ? 'bg-success shadow-[0_0_0_4px_color-mix(in_srgb,var(--success)_25%,transparent)]' : 'bg-faint')} />
      {s ? (
        <><span className="hidden min-[1440px]:inline">Till open · float {money(cfg.symbol, s.openingCash)} · in drawer</span>
          <span className="min-[1440px]:hidden">In drawer</span> <b className="tabular-nums">{money(cfg.symbol, d.summary.expected)}</b></>
      ) : (
        'Till closed'
      )}
    </span>
  )
}
