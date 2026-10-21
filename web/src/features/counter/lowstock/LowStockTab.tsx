import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Printer, Search } from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import type { ReorderRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { reorderQuery } from '@/features/purchasing/queries'
import { ReasonLine } from '@/features/purchasing/reorder/ReorderBits'
import { configQuery } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { suppliersQuery } from '../queries'
import { CounterPage, PaneTitle } from '../ui'
import { linesFor, savePrefill } from './prefill'

type Group = { supplierId: number; name: string; rows: ReorderRow[] }

function groups(rows: ReorderRow[]): Group[] {
  const by = new Map<number, Group>()
  for (const r of rows) {
    const g = by.get(r.supplierId) ?? { supplierId: r.supplierId, name: r.supplierName || 'No usual supplier', rows: [] }
    g.rows.push(r)
    by.set(r.supplierId, g)
  }
  return [...by.values()].sort((a, b) => (a.supplierId === 0 ? 1 : b.supplierId === 0 ? -1 : a.name.localeCompare(b.name)))
}

/** Low stock at the counter: what will run out before the next delivery, by
 * supplier — for the cashier who meets the reps. Ordering goes through the
 * Suppliers tab (Order these fills its Order job). */
export function LowStockTab() {
  const cfg = useSuspenseQuery(configQuery).data
  const [q, setQ] = useState('')
  const dq = useDeferredValue(q.trim())
  const data = useQuery({ ...reorderQuery({ q: dq, category: 0, supplier: 0, page: 1, all: true, till: true }), placeholderData: keepPreviousData }).data
  const gs = groups(data?.items ?? [])
  return (
    <CounterPage flush aside={<Aside canOrder={cfg.canHandleSuppliers} q={dq} count={data?.total ?? 0} />}>
      <PaneTitle title="Low stock" sub={data ? `${data.total} item${data.total === 1 ? '' : 's'}` : undefined} />
      <div className="relative px-4 pb-3">
        <Search size={16} className="pointer-events-none absolute left-7 top-1/2 -translate-y-[calc(50%+6px)] text-faint" />
        <input aria-label="Search low stock" placeholder="Name or barcode" value={q} onChange={(e) => setQ(e.target.value)} className={cn(FIELD, 'w-full pl-9')} />
      </div>
      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-auto px-4 pb-4">
        {data && gs.length === 0 && <p className="m-auto p-8 text-center text-faint">{dq ? 'Nothing matches' : 'Nothing is running low'}</p>}
        {gs.map((g) => <GroupBlock key={g.supplierId} g={g} canOrder={cfg.canHandleSuppliers} sym={cfg.symbol} />)}
      </div>
    </CounterPage>
  )
}

function GroupBlock({ g, canOrder, sym }: { g: Group; canOrder: boolean; sym: string }) {
  const navigate = useNavigate()
  const sups = useQuery({ ...suppliersQuery(''), enabled: canOrder && g.supplierId === 0 }).data ?? []
  const [pick, setPick] = useState(0)
  const supplierId = g.supplierId || pick
  const order = () => {
    savePrefill(supplierId, linesFor(g.rows))
    void navigate({ to: '/till/suppliers', search: { supplier: supplierId, job: 'order' } })
  }
  return (
    <section aria-label={g.name} className="shrink-0 overflow-hidden rounded-[var(--radius-sm)] border border-border">
      <header className="flex flex-wrap items-center gap-2 bg-muted px-3 py-2">
        <b className="min-w-0 flex-1 truncate">{g.name} <small className="font-medium text-muted-foreground">· {g.rows.length}</small></b>
        {canOrder && g.supplierId === 0 && (
          <select aria-label="Order from" value={pick} onChange={(e) => setPick(Number(e.target.value))} className={cn(FIELD, 'h-9 w-48 appearance-auto bg-card', !pick && 'border-warning')}>
            <option value={0}>— supplier —</option>
            {sups.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
        )}
        {canOrder && <Button className="h-9 px-3 font-bold" disabled={!supplierId} onClick={order}>Order these →</Button>}
      </header>
      <ul className="divide-y divide-border">
        {g.rows.map((r) => (
          <li key={r.productId} className="flex items-center gap-3 px-3 py-2.5">
            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
              <b className="flex items-center gap-1.5 truncate">{r.name}{num(r.onHand) <= 0 && <Tag tone="danger">out</Tag>}{r.calc?.notSelling && <Tag tone="muted">Not selling</Tag>}</b>
              <small className="text-[13px] text-faint tabular-nums">
                <span className="font-semibold text-danger">{qtyText(r.onHand)} {r.unitAbbr} left</span> · reorder at {r.reorderLevel}{r.cost && ` · cost ${money(sym, r.cost)}`}
              </small>
              <ReasonLine r={r} />
            </span>
            <span className="shrink-0 text-right">
              <small className="block text-[11px] text-faint">Order</small>
              <b className="font-display text-lg tabular-nums">{qtyText(r.suggested)}</b>
            </span>
          </li>
        ))}
      </ul>
    </section>
  )
}

function Aside({ canOrder, q, count }: { canOrder: boolean; q: string; count: number }) {
  const href = (all: boolean) => `/print/low-stock?till=true&q=${encodeURIComponent(q)}${all ? '&all=1' : ''}`
  return (
    <div className="flex flex-1 flex-col gap-4 p-5">
      <h2 className="font-display text-xl font-bold">What to order</h2>
      <p className="text-sm text-muted-foreground">
        Everything that will run out before its supplier comes again, with how much to order. Levels are learned every night from sales and deliveries.
      </p>
      {canOrder
        ? <p className="text-sm text-muted-foreground"><b className="text-foreground">Order these</b> on a supplier opens the Suppliers tab with the lines filled in — change anything, then place the order. Take the delivery in there too.</p>
        : <p className="text-sm text-muted-foreground">Print the list for the supplier's rep. To place orders here, ask the owner to tick Counter operations for you.</p>}
      <Button variant="outline" className="h-11 gap-2" disabled={!count} asChild>
        <a href={href(true)} target="_blank" rel="noreferrer"><Printer size={16} /> Print the list</a>
      </Button>
    </div>
  )
}
