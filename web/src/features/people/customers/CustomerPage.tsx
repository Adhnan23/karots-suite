import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowLeft, Ban, FileText, MoreHorizontal, Pencil, Scale } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Tag } from '@/features/admin/products/ProductsPage'
import { OpeningSheet } from '@/features/people/OpeningSheet'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { customerQuery } from '../queries'
import { CustomerForm } from './CustomerForm'
import { PaySheet } from './CustomerPay'
import { Due } from './CustomersPage'

type Sheet = 'pay' | 'edit' | 'opening' | 'disable' | null

/** One customer: what's due (or their advance), how much of the limit is used,
 * and the latest bills, returns and payments — collect, edit, old balance and
 * statement from here. */
export function CustomerPage({ id }: { id: number }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const c = useQuery(customerQuery(id)).data
  const qc = useQueryClient()
  const [sheet, setSheet] = useState<Sheet>(null)
  if (!c) return <EmptyState title="Loading…" />
  const owed = num(c.owed)
  const limit = num(c.creditLimit)
  const used = limit > 0 ? Math.min(Math.max(owed, 0) / limit, 1) : 0
  const toggle = async (on: boolean) => {
    await api(`/admin/customers/${id}${on ? '/activate' : ''}`, { method: on ? 'POST' : 'DELETE' })
    toast.success(on ? `${c.name} re-enabled` : `${c.name} disabled`)
    void qc.invalidateQueries({ queryKey: ['admin'] })
  }
  const act = 'h-10 gap-1.5 px-3.5'

  return (
    <>
      <Link to="/admin/customers" className="mb-2 inline-flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft size={14} /> Customers</Link>
      <header className={cn(SURFACE, 'mb-4 flex flex-col gap-4 p-5 md:flex-row md:items-end md:justify-between')}>
        <div className="min-w-0 md:max-w-md md:flex-1">
          <h1 className="flex flex-wrap items-center gap-2 font-display text-[24px] font-bold tracking-tight md:text-[28px]">
            {c.name}{!c.active && <Tag tone="muted">disabled</Tag>}{c.overLimit && <Tag tone="danger">over limit</Tag>}
          </h1>
          <p className="text-sm text-muted-foreground">{[c.phone, c.address].filter(Boolean).join(' · ')}{c.loyalty > 0 && ` · ${c.loyalty} points`} · since {new Date(c.since).toLocaleDateString([], { month: 'short', year: 'numeric' })}</p>
          <div className="mt-3">
            <small className="block text-[12px] font-semibold text-muted-foreground">{owed < 0 ? 'Advance they hold' : owed > 0 ? '↓ Due to us' : 'Balance'}</small>
            <Due sym={sym} owed={c.owed} big />
            {num(c.oldDebt) !== 0 && <small className="block text-muted-foreground tabular-nums">incl. old balance {money(sym, c.oldDebt)} from before the system</small>}
          </div>
          {limit > 0 && (
            <div className="mt-3" aria-label="Credit used">
              <div className="h-2 overflow-hidden rounded-full bg-muted"><div className={cn('h-full rounded-full', c.overLimit ? 'bg-danger' : used > 0.8 ? 'bg-warning' : 'bg-brand')} style={{ width: `${used * 100}%` }} /></div>
              <small className="mt-1 block text-muted-foreground tabular-nums">{c.overLimit ? `${money(sym, owed - limit)} over` : `${money(sym, limit - Math.max(owed, 0))} left`} of a {money(sym, limit)} limit</small>
            </div>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          {c.active && <Button className={cn(act, 'font-bold')} onClick={() => setSheet('pay')}>↓ Collect</Button>}
          <Button variant="outline" className={act} onClick={() => setSheet('edit')}><Pencil size={15} /> Edit</Button>
          <Button variant="outline" className={act} asChild><a href={`/print/statement/customer/${id}`} target="_blank" rel="noreferrer"><FileText size={15} /> Statement</a></Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild><Button variant="outline" className={act}><MoreHorizontal size={15} /> More</Button></DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => setSheet('opening')}><Scale size={15} /> Old balance</DropdownMenuItem>
              {c.active
                ? <DropdownMenuItem className="text-danger" onSelect={() => setSheet('disable')}><Ban size={15} /> Disable</DropdownMenuItem>
                : <DropdownMenuItem onSelect={() => void toggle(true).catch((e) => toast.error(errMsg(e)))}>Re-enable</DropdownMenuItem>}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>

      <section className={cn(SURFACE, 'overflow-hidden')}>
        <h2 className="border-b border-border px-4 py-2.5 text-[12px] font-bold uppercase tracking-wide text-faint">Latest activity</h2>
        {c.recent.length === 0 ? <p className="p-5 text-sm text-faint">No credit sales or payments yet.</p> : (
          <ul aria-label="Latest activity" className="divide-y divide-border">
            {c.recent.map((l, i) => {
              const inflow = num(l.credit) > 0
              return (
                <li key={i} className="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
                  <span className="min-w-0">
                    <b>{l.kind}</b>
                    <small className="block truncate text-faint">{new Date(l.date).toLocaleDateString([], { day: 'numeric', month: 'short', year: '2-digit' })}{l.ref && ` · ${l.ref}`} · balance {money(sym, l.balance)}</small>
                  </span>
                  <b className={cn('tabular-nums', inflow ? 'text-success' : 'text-foreground')}>{inflow ? '−' : '+'}{money(sym, inflow ? l.credit : l.debit)}</b>
                </li>
              )
            })}
          </ul>
        )}
      </section>

      {sheet === 'pay' && <PaySheet c={c} sym={sym} onClose={() => setSheet(null)} />}
      {sheet === 'edit' && <CustomerForm c={c} onClose={() => setSheet(null)} />}
      {sheet === 'opening' && <OpeningSheet kind="customer" id={id} name={c.name} owed={c.owed} oldDebt={c.oldDebt} sym={sym} onClose={() => setSheet(null)} />}
      {sheet === 'disable' && (
        <FormSheet open onOpenChange={(o) => !o && setSheet(null)} title={`Disable ${c.name}?`} submit="Disable" onSubmit={() => void toggle(false).then(() => setSheet(null), (e) => toast.error(errMsg(e)))}>
          <p className="text-sm text-muted-foreground">They drop off the till's customer search. Their history stays, and you can re-enable them any time.</p>
        </FormSheet>
      )}
    </>
  )
}
