import { ChevronDown } from 'lucide-react'
import type { ReactNode } from 'react'

const KEY = 'ks-fold'

function remembered(id: string): boolean | undefined {
  try { return (JSON.parse(localStorage.getItem(KEY) ?? '{}') as Record<string, boolean>)[id] } catch { return undefined }
}
function remember(id: string, open: boolean) {
  try { localStorage.setItem(KEY, JSON.stringify({ ...JSON.parse(localStorage.getItem(KEY) ?? '{}'), [id]: open })) } catch { /* private window: just don't remember */ }
}

/** A card that opens and closes (a native <details>). `remember` keeps it open or
 * closed on this device across visits; `hint` sits beside the title (e.g. the current pick). */
export function Fold({ title, hint, open, remember: id, children }: { title: string; hint?: ReactNode; open?: boolean; remember?: string; children: ReactNode }) {
  const start = (id ? remembered(id) : undefined) ?? open
  return (
    <details open={start} onToggle={id ? (e) => remember(id, e.currentTarget.open) : undefined}
      className="group rounded-[var(--radius)] border border-border bg-card shadow-[var(--shadow)] [backdrop-filter:var(--glass)]">
      <summary className="flex cursor-pointer list-none items-center gap-2 px-4 py-3 [&::-webkit-details-marker]:hidden">
        <span className="font-display font-bold">{title}</span>
        {hint && <span className="min-w-0 truncate text-xs font-semibold text-faint">{hint}</span>}
        <ChevronDown className="ml-auto size-4 shrink-0 text-muted-foreground transition-transform duration-(--dur-fast) group-open:rotate-180" />
      </summary>
      <div className="flex flex-col gap-3 border-t border-border px-4 py-3">{children}</div>
    </details>
  )
}
