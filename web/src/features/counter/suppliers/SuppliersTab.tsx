import { keepPreviousData, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { ExternalLink, Plus, Search } from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { NewSupplierRequest, SupplierRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { configQuery, errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { supplierQuery, suppliersQuery } from '../queries'
import { CounterPage, PaneTitle } from '../ui'
import { PayForm } from './PayForm'
import { StockForm } from './StockForm'
import { SupplierClaimsPanel } from '@/features/purchasing/SupplierClaims'
import { slots } from '@/plugins/slots'

type Job = 'pay' | 'receive' | 'order' | `x:${string}` // x: = a plugin's job (slots.supplierJobs)

/** Suppliers at the counter: pay what the shop owes, take in a delivery, place an order. */
export function SuppliersTab() {
  const open = getRouteApi('/_authed/till/suppliers').useSearch() // from Low stock: a supplier's Order, lines filled in
  const [id, setId] = useState<number | null>(open.supplier ?? null)
  return (
    <CounterPage flush aside={id ? <Workspace key={id} id={id} startJob={id === open.supplier ? open.job : undefined} /> : (
      <p className="m-auto max-w-[280px] p-8 text-center text-faint">Pick a supplier — or add one if they're new.</p>
    )}>
      <SupplierList picked={id} onPick={setId} />
    </CounterPage>
  )
}

function SupplierList({ picked, onPick }: { picked: number | null; onPick: (id: number) => void }) {
  const cfg = useSuspenseQuery(configQuery).data
  const qc = useQueryClient()
  const [q, setQ] = useState('')
  const [adding, setAdding] = useState(false)
  const dq = useDeferredValue(q.trim())
  const rows = useQuery({ ...suppliersQuery(dq), placeholderData: keepPreviousData }).data ?? []

  async function add(e: { preventDefault(): void; currentTarget: HTMLFormElement }) {
    e.preventDefault()
    const f = new FormData(e.currentTarget)
    try {
      const body: NewSupplierRequest = { name: String(f.get('sup_name') ?? ''), phone: String(f.get('sup_phone') ?? '') }
      const s = await api<SupplierRow>('/till/suppliers', { json: body })
      toast.success(`${s.name} added`)
      setAdding(false)
      void qc.invalidateQueries({ queryKey: ['till', 'suppliers'] })
      onPick(s.id)
    } catch (err) {
      toast.error(errMsg(err))
    }
  }

  return (
    <>
      <PaneTitle title="Suppliers" sub={`${rows.length} shown`}
        right={<Button size="sm" variant="secondary" onClick={() => setAdding((a) => !a)}><Plus size={14} /> New</Button>} />
      {adding && (
        <form onSubmit={(e) => void add(e)} autoComplete="off" className="mx-3 mb-2 flex gap-1.5 rounded-[var(--radius-sm)] bg-muted p-2">
          <input autoFocus name="sup_name" aria-label="Supplier name" placeholder="Supplier name" required className={cn(FIELD, 'min-w-0 flex-1 bg-card')} />
          <input name="sup_phone" aria-label="Supplier phone" placeholder="Phone" inputMode="tel" className={cn(FIELD, 'w-32 bg-card')} />
          <Button type="submit">Add</Button>
        </form>
      )}
      <div className="relative mx-3 mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input autoFocus={!adding} aria-label="Find a supplier" placeholder="Name or phone" autoComplete="off" value={q}
          onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && rows[0] && onPick(rows[0].id)}
          className={cn(FIELD, 'w-full pl-9')} />
      </div>
      <ul aria-label="Suppliers" className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-2.5 pb-2.5">
        {rows.length === 0 && <li className="m-auto p-8 text-center text-faint">No supplier matches.</li>}
        {rows.map((s) => {
          const owed = num(s.owed)
          return (
            <li key={s.id}>
              <button type="button" aria-pressed={picked === s.id} onClick={() => onPick(s.id)}
                className="flex w-full items-center gap-3 rounded-[var(--radius-sm)] px-3 py-2 text-left hover:bg-muted aria-pressed:bg-selected aria-pressed:text-selected-foreground">
                <span className="min-w-0 flex-1">
                  <b className="block truncate text-[14px]">{s.name}</b>
                  <small className="opacity-70">{s.phone || s.contact || 'no phone'}</small>
                </span>
                {owed !== 0 && (
                  <span className="text-right">
                    <b className={cn('block tabular-nums', picked !== s.id && (owed > 0 ? 'text-warning' : 'text-success'))}>{money(cfg.symbol, Math.abs(owed))}</b>
                    <small className="opacity-70">{owed > 0 ? 'we owe' : 'advance'}</small>
                  </span>
                )}
              </button>
            </li>
          )
        })}
      </ul>
    </>
  )
}

function Workspace({ id, startJob }: { id: number; startJob?: Job }) {
  const cfg = useSuspenseQuery(configQuery).data
  const s = useQuery(supplierQuery(id)).data
  const [job, setJob] = useState<Job>(startJob ?? 'pay')
  const [orderId, setOrderId] = useState<number | null>(null)
  if (!s) return null
  const owed = num(s.owed)
  const Plugin = slots.supplierJobs.find((j) => `x:${j.key}` === job)?.Panel
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <header className="flex items-start justify-between gap-3 px-5 pb-3 pt-5">
        <div className="min-w-0">
          <h2 className="truncate font-display text-xl font-bold">{s.name}</h2>
          <p className="text-[13px] text-muted-foreground">
            {owed > 0 ? `We owe ${money(cfg.symbol, owed)}` : owed < 0 ? `They hold ${money(cfg.symbol, -owed)} of ours` : 'All settled'}
            {s.creditDays > 0 && ` · ${s.creditDays} days credit`}
          </p>
        </div>
        <Seg label="Job" value={job} onChange={(j) => { setJob(j); setOrderId(null) }} options={[['pay', owed < 0 ? 'Pay / refund' : 'Pay'], ['receive', 'Receive stock'], ['order', 'Order'],
          ...slots.supplierJobs.map((j): [Job, string] => [`x:${j.key}`, j.label])]} />
      </header>
      {job === 'receive' && s.orders.length > 0 && (
        <div className="mx-5 mb-3 flex flex-wrap items-center gap-1.5 rounded-[var(--radius-sm)] bg-muted p-2 text-[13px]">
          <span className="px-1 font-semibold text-muted-foreground">Against an order:</span>
          <button type="button" aria-pressed={orderId === null} onClick={() => setOrderId(null)}
            className="rounded-pill border border-border bg-card px-2.5 py-1 font-semibold aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground">No order</button>
          {s.orders.map((o) => (
            <button key={o.id} type="button" aria-pressed={orderId === o.id} onClick={() => setOrderId(o.id)}
              className="rounded-pill border border-border bg-card px-2.5 py-1 font-semibold tabular-nums aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground">
              #{o.id} · {new Date(o.date).toLocaleDateString([], { day: 'numeric', month: 'short' })}
            </button>
          ))}
          {orderId && <a href={`/print/order/${orderId}`} target="_blank" rel="noreferrer" className="ml-auto flex items-center gap-1 text-muted-foreground hover:text-foreground">Print <ExternalLink size={13} /></a>}
        </div>
      )}
      <SupplierClaimsPanel supplierId={s.id} sym={cfg.symbol} base="/till" className="mx-5 mb-3" />
      {job === 'pay' && <PayForm s={s} />}
      {job === 'receive' && <StockForm key={`r-${orderId ?? 0}`} mode="receive" supplier={s} orderId={orderId} />}
      {job === 'order' && <StockForm key="o" mode="order" supplier={s} orderId={null} />}
      {Plugin && <Plugin supplier={s} />}
    </div>
  )
}
