import { TriangleAlert } from 'lucide-react'
import type { SaleView } from '@/api/types.gen'
import { stamp } from '@/features/money/format'

const time = (iso: string) => new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

/** Where a return's cash comes from by default: the till that took the sale, while it's still open. */
export const returnSource = (sale: SaleView) => sale.tillSource ?? ''

/** "Needs review": what the server took instead of refusing (a sale rung away
 * from it), and when it was really rung if that wasn't when it counts. */
export function ReviewBlock({ sale }: { sale: SaleView }) {
  const flags = sale.flags ?? []
  const rung = sale.rungAt && Math.abs(Date.parse(sale.rungAt) - Date.parse(sale.createdAt)) > 60_000 ? sale.rungAt : ''
  if (!flags.length && !rung) return null
  return (
    <section aria-label="Needs review" className="flex flex-col gap-1 rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] px-3 py-2.5 text-sm">
      {flags.length > 0 && <b className="flex items-center gap-1.5 text-warning"><TriangleAlert size={15} />Needs review</b>}
      {flags.map((f, i) => (
        <span key={i} className="flex justify-between gap-3">
          <span className="min-w-0">{f.note || f.kind}</span>
          <small className="shrink-0 text-muted-foreground tabular-nums">{stamp(f.createdAt)}</small>
        </span>
      ))}
      {rung && <span className="text-muted-foreground">Rung at {time(rung)} · counted {stamp(sale.createdAt)}</span>}
    </section>
  )
}
