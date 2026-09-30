import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft, FileText, PackageCheck, Pencil, Trash2, Undo2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { configQuery } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { purchaseQuery } from '../queries'
import { day, StatusTag } from './PurchasesPage'

/** One order or delivery: lines (ordered vs received), discounts and what's
 * paid, credited and still owed — receive / edit / print from here. */
export function PurchasePage({ id }: { id: number }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const p = useQuery(purchaseQuery(id)).data
  const qc = useQueryClient()
  const go = useNavigate()
  if (!p) return <EmptyState title="Loading…" />
  const draft = p.status === 'draft'
  const act = 'h-10 gap-1.5 px-3.5'
  const row = (label: string, v: string, strong?: boolean) => (
    <div className={cn('flex justify-between gap-4', strong ? 'text-base font-bold' : 'text-muted-foreground')}><dt>{label}</dt><dd className="tabular-nums">{v}</dd></div>
  )
  return (
    <>
      <Link to="/admin/purchases" search={{ tab: draft ? 'orders' : 'received' }} className="mb-2 inline-flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft size={14} /> Purchases</Link>
      <header className={cn(SURFACE, 'mb-4 flex flex-col gap-4 p-5 md:flex-row md:items-end md:justify-between')}>
        <div className="min-w-0">
          <p className="text-[12px] font-bold uppercase tracking-wide text-faint">{draft ? `Purchase order #${p.id}` : `Delivery${p.invoiceNo ? ` · invoice ${p.invoiceNo}` : ` #${p.id}`}`}</p>
          <h1 className="flex flex-wrap items-center gap-2 font-display text-[24px] font-bold tracking-tight md:text-[28px]">
            <Link to="/admin/suppliers/$id" params={{ id: String(p.supplierId) }} className="hover:underline">{p.supplierName}</Link><StatusTag p={p} />
          </h1>
          <p className="text-sm text-muted-foreground">
            {draft ? 'Ordered' : 'Received'} {day(p.date)}{p.by && ` by ${p.by}`}{draft && p.expectedDate && ` · expected ${day(p.expectedDate)}`}{!draft && p.dueDate && ` · pay by ${day(p.dueDate)}`}
          </p>
          {p.notes && <p className="mt-1 text-sm">{p.notes}</p>}
        </div>
        <div className="flex flex-wrap gap-2">
          {draft ? (
            <>
              <Button className={cn(act, 'font-bold')} asChild><Link to="/admin/purchases/$id/receive" params={{ id: String(id) }}><PackageCheck size={16} /> Receive</Link></Button>
              <Button variant="outline" className={act} asChild><Link to="/admin/purchases/$id/edit" params={{ id: String(id) }}><Pencil size={15} /> Edit</Link></Button>
              <Button variant="outline" className={act} asChild><a href={`/print/po?ids=${id}`} target="_blank" rel="noreferrer"><FileText size={15} /> Print order</a></Button>
              <ConfirmButton label={<><Trash2 size={15} /> Delete</>} title={`Delete order #${id}?`} body="Nothing came in on it, so nothing else changes." confirm="Delete" className={act}
                onConfirm={async () => {
                  await api(`/admin/purchases/${id}`, { method: 'DELETE' })
                  toast.success(`Order #${id} deleted`)
                  void qc.invalidateQueries({ queryKey: ['admin'] })
                  void go({ to: '/admin/purchases' })
                }} />
            </>
          ) : (
            <>
              {num(p.balance) > 0 && <Button className={cn(act, 'font-bold')} asChild><Link to="/admin/suppliers/$id" params={{ id: String(p.supplierId) }}>↑ Pay {p.supplierName}</Link></Button>}
              <Button variant="outline" className={act} asChild><Link to="/admin/purchase-returns/new" search={{ purchase: id }}><Undo2 size={15} /> Return goods</Link></Button>
              <Button variant="outline" className={act} asChild><a href={`/print/grn/${id}`} target="_blank" rel="noreferrer"><FileText size={15} /> Goods received note</a></Button>
            </>
          )}
        </div>
      </header>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_300px]">
        <section className={cn(SURFACE, 'overflow-hidden')}>
          <ul aria-label="Lines" className="divide-y divide-border">
            {p.lines.map((l) => {
              const diff = !draft && l.ordered ? num(l.quantity) - num(l.ordered) : 0
              return (
                <li key={l.productId} className="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
                  <span className="min-w-0">
                    <b className="block truncate">{l.name}</b>
                    <small className="text-faint tabular-nums">
                      {draft ? `${qtyText(l.quantity)} ${l.unitAbbr}` : l.ordered ? `${qtyText(l.quantity)} of ${qtyText(l.ordered)} ordered` : `${qtyText(l.quantity)} ${l.unitAbbr} · not on the order`}
                      {num(l.freeQty) > 0 && ` + ${qtyText(l.freeQty)} free`} · {money(sym, l.costPrice)} each
                      {num(l.discount) > 0 && ` · −${l.discountType === 'percent' ? `${l.discount}%` : `${money(sym, l.discount)}/unit`}`}
                      {l.expiryDate && ` · exp ${l.expiryDate}`}
                    </small>
                    {diff !== 0 && <small className={cn('ml-2 font-bold', diff < 0 ? 'text-warning' : 'text-brand')}>{diff < 0 ? `${qtyText(-diff)} short` : `${qtyText(diff)} extra`}</small>}
                  </span>
                  <b className="tabular-nums">{money(sym, l.net)}</b>
                </li>
              )
            })}
          </ul>
        </section>
        <dl className={cn(SURFACE, 'flex flex-col gap-1.5 self-start p-4 text-sm')}>
          {row('Lines', money(sym, p.subtotal))}
          {num(p.discount) > 0 && row(`Bill discount${p.discountType === 'percent' ? ` (${p.discountValue}%)` : ''}`, `−${money(sym, p.discount)}`)}
          {row('Total', money(sym, p.total), true)}
          {!draft && (
            <>
              {num(p.paid) > 0 && row('Paid', `−${money(sym, p.paid)}`)}
              {num(p.credited) > 0 && row('Returned (credited)', `−${money(sym, p.credited)}`)}
              {row('Still owed', money(sym, Math.max(num(p.balance), 0)), true)}
            </>
          )}
        </dl>
      </div>
    </>
  )
}
