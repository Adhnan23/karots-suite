import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { configQuery } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { day } from '../purchases/PurchasesPage'
import { purchaseReturnQuery } from '../queries'

/** A posted debit note — read-only: it already moved the stock and the payable,
 * so a mistake is corrected with an opposing entry. */
export function ReturnPage({ id }: { id: number }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const r = useQuery(purchaseReturnQuery(id)).data
  if (!r) return <EmptyState title="Loading…" />
  return (
    <>
      <Link to="/admin/purchase-returns" className="mb-2 inline-flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft size={14} /> Purchase returns</Link>
      <header className={cn(SURFACE, 'mb-4 flex flex-col gap-1 p-5')}>
        <p className="text-[12px] font-bold uppercase tracking-wide text-faint">Debit note {r.reference || `#${r.id}`}</p>
        <h1 className="font-display text-[24px] font-bold tracking-tight md:text-[28px]">
          <Link to="/admin/suppliers/$id" params={{ id: String(r.supplierId) }} className="hover:underline">{r.supplierName}</Link>
        </h1>
        <p className="text-sm text-muted-foreground">Returned {day(r.date)}{r.reason && ` · ${r.reason}`}
          {r.purchaseId > 0 && <> · from <Link to="/admin/purchases/$id" params={{ id: String(r.purchaseId) }} className="font-semibold text-brand">{r.invoiceNo || `GRN #${r.purchaseId}`}</Link></>}</p>
        <b className="mt-2 font-display text-3xl text-success tabular-nums">↓ {money(sym, r.total)}</b>
        <small className="text-muted-foreground">{r.purchaseId > 0 ? "credited to that delivery's bill first" : 'credited against their oldest open bills'}</small>
      </header>
      <section className={cn(SURFACE, 'overflow-hidden')}>
        <ul aria-label="Lines" className="divide-y divide-border">
          {r.lines.map((l, i) => (
            <li key={i} className="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
              <span className="min-w-0"><b className="block truncate">{l.name}</b><small className="text-faint tabular-nums">{qtyText(l.quantity)} × {money(sym, l.costPrice)}{r.purchaseId > 0 ? ' · real cost after discounts' : ''}</small></span>
              <b className="tabular-nums">{money(sym, l.subtotal)}</b>
            </li>
          ))}
        </ul>
      </section>
    </>
  )
}
