import { CircleCheck, TriangleAlert } from 'lucide-react'
import type { CheckFinding } from '@/api/types.gen'

/** The data check's findings: stored data older karots-pos versions wrote
 * wrongly. Read-only — it says what is off and why; the owner decides. */
export function DataCheckList({ findings }: { findings: CheckFinding[] }) {
  if (findings.length === 0) {
    return (
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <CircleCheck size={18} className="shrink-0 text-success" /> Nothing found — the stored figures add up.
      </p>
    )
  }
  return (
    <ul aria-label="Data check findings" className="grid gap-2">
      {findings.map((f) => (
        <li key={f.key}>
          <details className="group rounded-[calc(var(--radius)-4px)] border border-border bg-background/40">
            <summary className="flex cursor-pointer list-none items-start gap-2.5 p-3">
              <TriangleAlert size={18} className="mt-0.5 shrink-0 text-warning" />
              <span className="min-w-0 flex-1 text-sm font-semibold">{f.title}</span>
              <span className="shrink-0 rounded-full bg-warning/15 px-2 py-0.5 text-xs font-bold tabular-nums text-warning">{f.count}</span>
            </summary>
            <div className="border-t border-border px-3 pb-3 pt-2 text-sm">
              <p className="mb-2 text-muted-foreground">{f.explain}</p>
              <ul className="grid gap-1">
                {f.sample.map((r, i) => (
                  <li key={i} className="flex flex-wrap gap-x-2 break-words"><b className="tabular-nums">{r.ref}</b><span className="min-w-0 text-muted-foreground">{r.detail}</span></li>
                ))}
              </ul>
              {f.count > f.sample.length && <p className="mt-1.5 text-xs text-faint">and {f.count - f.sample.length} more</p>}
            </div>
          </details>
        </li>
      ))}
    </ul>
  )
}
