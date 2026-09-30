import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowLeft, Ban, FileText, MoreHorizontal, MoveRight, Pencil, Scale, Undo2 } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminSupplierDetail, CountResult, ReassignRequest } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Tag } from '@/features/admin/products/ProductsPage'
import { OpeningSheet } from '@/features/people/OpeningSheet'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { suppliersQuery as allSuppliersQuery } from '@/features/counter/queries'
import { supplierQuery } from '../queries'
import { SupplierForm } from './SupplierForm'
import { PaySheet, RefundSheet } from './SupplierMoney'
import { Owed } from './SuppliersPage'

type Sheet = 'pay' | 'refund' | 'edit' | 'opening' | 'move' | 'disable' | null
const day = (iso: string) => new Date(iso).toLocaleDateString([], { day: 'numeric', month: 'short', year: '2-digit' })

function Section({ title, count, children }: { title: string; count?: number; children: ReactNode }) {
  return (
    <section className={cn(SURFACE, 'overflow-hidden')}>
      <h2 className="flex items-center gap-2 border-b border-border px-4 py-2.5 text-[12px] font-bold uppercase tracking-wide text-faint">
        {title}{count !== undefined && <span className="rounded-full bg-muted px-2 text-foreground">{count}</span>}
      </h2>
      {children}
    </section>
  )
}

/** One supplier: what's owed either way, the open bills, orders on the way and
 * every payment / refund — with pay, refund, edit, old balance, statement and
 * move-products from here. */
export function SupplierPage({ id }: { id: number }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const s = useQuery(supplierQuery(id)).data
  const qc = useQueryClient()
  const [sheet, setSheet] = useState<Sheet>(null)
  if (!s) return <EmptyState title="Loading…" />
  const owed = num(s.owed)
  const today = new Date().toISOString().slice(0, 10)
  const toggle = async (on: boolean) => {
    await api(`/admin/suppliers/${id}${on ? '/activate' : ''}`, { method: on ? 'POST' : 'DELETE' })
    toast.success(on ? `${s.name} re-enabled` : `${s.name} disabled`)
    void qc.invalidateQueries({ queryKey: ['admin'] })
  }
  const act = 'h-10 gap-1.5 px-3.5'

  return (
    <>
      <Link to="/admin/suppliers" className="mb-2 inline-flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft size={14} /> Suppliers</Link>
      <header className={cn(SURFACE, 'mb-4 flex flex-col gap-4 p-5 md:flex-row md:items-end md:justify-between')}>
        <div className="min-w-0">
          <h1 className="flex flex-wrap items-center gap-2 font-display text-[24px] font-bold tracking-tight md:text-[28px]">
            {s.name}{!s.active && <Tag tone="muted">disabled</Tag>}
          </h1>
          <p className="text-sm text-muted-foreground">{[s.contact, s.phone, s.address].filter(Boolean).join(' · ') || 'No contact details'} · {s.creditDays ? `${s.creditDays} days credit` : 'pays on delivery'}</p>
          <div className="mt-3">
            <small className="block text-[12px] font-semibold text-muted-foreground">{owed < 0 ? '↓ They owe us' : owed > 0 ? '↑ We owe them' : 'Balance'}</small>
            <Owed sym={sym} owed={s.owed} big />
            {num(s.oldDebt) !== 0 && <small className="block text-muted-foreground tabular-nums">incl. old balance {money(sym, s.oldDebt)} from before the system</small>}
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {s.active && owed >= 0 && <Button className={cn(act, 'font-bold')} onClick={() => setSheet('pay')}>↑ Pay</Button>}
          {s.active && owed < 0 && <Button className={cn(act, 'font-bold')} onClick={() => setSheet('refund')}>↓ Take refund</Button>}
          <Button variant="outline" className={act} onClick={() => setSheet('edit')}><Pencil size={15} /> Edit</Button>
          <Button variant="outline" className={act} asChild><a href={`/print/statement/supplier/${id}`} target="_blank" rel="noreferrer"><FileText size={15} /> Statement</a></Button>
          {s.active ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild><Button variant="outline" className={act}><MoreHorizontal size={15} /> More</Button></DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {owed < 0 && <DropdownMenuItem onSelect={() => setSheet('pay')}>Pay an advance</DropdownMenuItem>}
                <DropdownMenuItem onSelect={() => setSheet('opening')}><Scale size={15} /> Old balance</DropdownMenuItem>
                <DropdownMenuItem asChild><Link to="/admin/purchase-returns/new" search={{ supplier: id }}><Undo2 size={15} /> Return goods</Link></DropdownMenuItem>
                <DropdownMenuItem onSelect={() => setSheet('move')}><MoveRight size={15} /> Move products ({s.products})</DropdownMenuItem>
                <DropdownMenuItem className="text-danger" onSelect={() => setSheet('disable')}><Ban size={15} /> Disable</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <>
              <Button variant="outline" className={act} onClick={() => setSheet('opening')}><Scale size={15} /> Old balance</Button>
              <Button variant="outline" className={act} onClick={() => void toggle(true).catch((e) => toast.error(errMsg(e)))}>Re-enable</Button>
            </>
          )}
        </div>
      </header>

      <div className="grid gap-4 lg:grid-cols-2">
        <Section title="Open bills" count={s.invoices.length}>
          {s.invoices.length === 0 ? <p className="p-5 text-sm text-faint">Nothing unpaid.</p> : (
            <ul className="divide-y divide-border">
              {s.invoices.map((i) => (
                <li key={i.id} className="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
                  <span className="min-w-0">
                    <b className="flex items-center gap-1.5">{i.invoiceNo || `#${i.id}`}{i.dueDate && i.dueDate < today && <Tag tone="danger">overdue</Tag>}</b>
                    <small className="text-faint">{day(i.date)}{i.dueDate && ` · due ${i.dueDate}`} · bill {money(sym, i.total)}</small>
                  </span>
                  <b className="tabular-nums">{money(sym, i.balance)}</b>
                </li>
              ))}
            </ul>
          )}
        </Section>
        <Section title="Ordered, not in yet" count={s.orders.length}>
          {s.orders.length === 0 ? <p className="p-5 text-sm text-faint">No open orders.</p> : (
            <ul className="divide-y divide-border">
              {s.orders.map((o) => (
                <li key={o.id} className="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
                  <span className="min-w-0"><b>Order #{o.id}</b><small className="block truncate text-faint">{day(o.date)}{o.notes && ` · ${o.notes}`}</small></span>
                  <a href={`/print/order/${o.id}`} target="_blank" rel="noreferrer" className="text-[13px] font-semibold text-brand">Print</a>
                </li>
              ))}
            </ul>
          )}
        </Section>
        <div className="lg:col-span-2">
          <Section title="Payments and refunds" count={s.money.length}>
            {s.money.length === 0 ? <p className="p-5 text-sm text-faint">No money has moved yet.</p> : (
              <ul aria-label="Payments and refunds" className="divide-y divide-border">
                {s.money.map((m, i) => (
                  <li key={i} className="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
                    <span className="min-w-0">
                      <b>{m.kind === 'refund' ? '↓ Refund from them' : '↑ Paid to them'}</b>
                      <small className="block truncate text-faint">{day(m.date)} · {m.method}{m.reference && ` · ${m.reference}`}{m.by && ` · ${m.by}`}{m.note && ` · ${m.note}`}</small>
                    </span>
                    <b className={cn('tabular-nums', m.kind === 'refund' ? 'text-success' : 'text-danger')}>{money(sym, m.amount)}</b>
                  </li>
                ))}
              </ul>
            )}
          </Section>
        </div>
      </div>

      {sheet === 'pay' && <PaySheet s={s} sym={sym} onClose={() => setSheet(null)} />}
      {sheet === 'refund' && <RefundSheet s={s} sym={sym} onClose={() => setSheet(null)} />}
      {sheet === 'edit' && <SupplierForm s={s} onClose={() => setSheet(null)} />}
      {sheet === 'opening' && <OpeningSheet kind="supplier" id={id} name={s.name} owed={s.owed} oldDebt={s.oldDebt} sym={sym} onClose={() => setSheet(null)} />}
      {sheet === 'move' && <MoveSheet s={s} onClose={() => setSheet(null)} />}
      {sheet === 'disable' && (
        <FormSheet open onOpenChange={(o) => !o && setSheet(null)} title={`Disable ${s.name}?`} submit="Disable" onSubmit={() => void toggle(false).then(() => setSheet(null), (e) => toast.error(errMsg(e)))}>
          <p className="text-sm text-muted-foreground">They drop off pickers and the counter. Their history stays, and you can re-enable them any time.</p>
        </FormSheet>
      )}
    </>
  )
}

/** A distributor change: every product defaulting to this supplier moves to another. */
function MoveSheet({ s, onClose }: { s: AdminSupplierDetail; onClose: () => void }) {
  const qc = useQueryClient()
  const all = useQuery(allSuppliersQuery('')).data?.filter((x) => x.id !== s.id) ?? []
  const [to, setTo] = useState(0)
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      const r = await api<CountResult>(`/admin/suppliers/${s.id}/reassign`, { json: { toId: to } satisfies ReassignRequest })
      toast.success(`Moved ${r.count} product${r.count === 1 ? '' : 's'}`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Move products to another supplier" submit="Move them" busy={busy || !to || s.products === 0} onSubmit={save}>
      <p className="text-sm text-muted-foreground"><b className="text-foreground">{s.products}</b> product{s.products === 1 ? '' : 's'} default to {s.name}. Pick who delivers them now — every one moves across.</p>
      <Field label="Move all to" hint="Only the default supplier changes — stock, prices and history stay put.">
        <select aria-label="Move all to" value={to} onChange={(e) => setTo(Number(e.target.value))} className={cn(INPUT, 'appearance-auto')}>
          <option value={0}>— pick a supplier —</option>
          {all.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
        </select>
      </Field>
    </FormSheet>
  )
}
