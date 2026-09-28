import { useQuery } from '@tanstack/react-query'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery } from '../api'

/** Top-bar drawer status: "Till open · float Rs … · in drawer Rs …". */
export function DrawerPill() {
  const cfg = useQuery(configQuery).data
  const d = useQuery(drawerQuery).data
  if (!cfg || !d) return null
  const s = d.summary.session
  return (
    <span className="flex items-center gap-2 whitespace-nowrap rounded-full border border-border bg-muted px-3 py-1.5 text-[13px]">
      <span className={cn('h-2 w-2 rounded-full', s ? 'bg-success shadow-[0_0_0_4px_color-mix(in_srgb,var(--success)_25%,transparent)]' : 'bg-faint')} />
      {s ? (
        <>Till open · float {money(cfg.symbol, s.openingCash)} · in drawer <b className="tabular-nums">{money(cfg.symbol, d.summary.expected)}</b></>
      ) : (
        'Till closed'
      )}
    </span>
  )
}
