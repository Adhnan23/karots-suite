import { useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { AdminExpenseRow } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { countText } from '@/features/counter/queries'
import { RangePicker } from '@/features/counter/RangePicker'
import { configQuery } from '@/features/till/api'
import { money } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { expenseSummaryQuery, expensesQuery } from '../queries'
import { MethodBadge } from '@/components/admin/MethodBadge'
import { ExpenseForm, PayAgainSheet, VoidSheet } from './ExpenseForm'

const route = getRouteApi('/_authed/admin/expenses/')
const day = (d: string) => new Date(`${d}T00:00`).toLocaleDateString([], { day: 'numeric', month: 'short' })

/** Expenses (karots-pos /admin/expenses): what the shop spent in a range, by
 * category, paged 50 at a time; record, edit, or void one. */
export function ExpensesPage() {
  const s = route.useSearch()
  const f = { preset: s.preset ?? 'this-month', from: s.from ?? '', to: s.to ?? '', category: s.category ?? '', q: s.q ?? '' }
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof f>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q, 250)
  useEffect(() => { if (dq !== f.q) set({ q: dq }) }, [dq]) // eslint-disable-line react-hooks/exhaustive-deps
  const sum = useQuery(expenseSummaryQuery(f)).data
  const list = useInfiniteQuery(expensesQuery(f))
  const rows = list.data?.pages.flat() ?? []
  const [open, setOpen] = useState<{ kind: 'new' | 'edit' | 'void' | 'pay'; e?: AdminExpenseRow } | null>(null)
  const chip = 'flex items-baseline gap-1.5 whitespace-nowrap rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

  return (
    <>
      <PageHead title="Expenses" sub="What the shop spent — rent, bills, wages, transport."
        actions={<Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setOpen({ kind: 'new' })}>↑ Record expense</Button>} />
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <RangePicker full value={f} onChange={(r) => set(r)} />
        <div className="relative md:w-72">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search expenses" placeholder="Category or note" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
      </div>
      <section className={cn(SURFACE, 'mb-3 flex flex-col gap-3 p-4 md:flex-row md:items-center')}>
        <div className="md:min-w-56">
          <small className="block text-[12px] font-semibold text-muted-foreground">↑ Spent{f.category ? ` on ${f.category}` : ''}{f.q ? ' (matching)' : ''}</small>
          <b className="font-display text-3xl font-extrabold tabular-nums text-danger">{sum ? money(sym, sum.total) : '…'}</b>
          <small className="block text-[12px] text-muted-foreground">{sum ? `${sum.count} expense${sum.count === 1 ? '' : 's'}` : ''}</small>
        </div>
        {!!sum?.byCategory.length && (
          <div role="group" aria-label="Categories" className="-mx-1 flex gap-1.5 overflow-x-auto px-1 pb-1 md:flex-wrap md:overflow-visible">
            <button type="button" aria-pressed={!f.category} onClick={() => set({ category: '' })} className={chip}>All</button>
            {sum.byCategory.map((c) => (
              <button key={c.category} type="button" aria-pressed={f.category === c.category} onClick={() => set({ category: f.category === c.category ? '' : c.category })} className={chip}>
                {c.category} <small className="tabular-nums opacity-75">{money(sym, c.total)}</small>
              </button>
            ))}
          </div>
        )}
      </section>
      <div className="mb-2 text-right text-[13px] text-muted-foreground">{countText(rows.length, !!list.hasNextPage)} shown</div>
      <DataList label="Expenses" rows={rows} rowKey={(e) => e.id} onRow={(e) => setOpen({ kind: 'edit', e })}
        empty={<EmptyState title={list.isPending ? 'Loading…' : 'No expenses in this range'}>{!list.isPending && !f.q && !f.category && 'Record rent, bills and other costs here, and say which locker or till paid them.'}</EmptyState>}
        footer={list.hasNextPage && (
          <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
            className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60">
            {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
          </button>
        )}
        columns={[
          { key: 'what', header: 'Expense', cell: (e) => (
            <span className="flex flex-col">
              <b>{e.category}</b>
              <small className="font-normal text-faint">{day(e.date)}{e.description && ` · ${e.description}`}</small>
            </span>
          ) },
          { key: 'from', header: 'Paid from', cell: (e) => <PaidFrom e={e} onPay={() => setOpen({ kind: 'pay', e })} /> },
          { key: 'by', header: 'By', wide: true, cell: (e) => <span className="text-muted-foreground">{e.paidBy || '—'}</span> },
          { key: 'svc', header: 'Service', wide: true, cell: (e) => <span className="text-muted-foreground">{e.serviceName || '—'}</span> },
          { key: 'amount', header: 'Amount', num: true, cell: (e) => <b className="tabular-nums text-danger">↑ {money(sym, e.amount)}</b> },
        ]} />
      {open?.kind === 'new' && <ExpenseForm meta={sum} sym={sym} onClose={() => setOpen(null)} />}
      {open?.kind === 'edit' && open.e && <ExpenseForm e={open.e} meta={sum} sym={sym} onClose={() => setOpen(null)} onVoid={() => setOpen({ kind: 'void', e: open.e })} />}
      {open?.kind === 'void' && open.e && <VoidSheet e={open.e} sym={sym} onClose={() => setOpen(null)} />}
      {open?.kind === 'pay' && open.e && <PayAgainSheet e={open.e} sym={sym} onClose={() => setOpen(null)} />}
    </>
  )
}

/** Where an expense's money came from — or that its cheque is still out, or
 *  came back and it needs paying again. */
function PaidFrom({ e, onPay }: { e: AdminExpenseRow; onPay: () => void }) {
  if (Number(e.unpaid) > 0)
    return (
      <span className="flex flex-wrap items-center gap-1.5">
        <span className="font-semibold text-warning">Not paid — cheque {e.unpaidWhy || 'returned'}</span>
        <button type="button" onClick={(x) => { x.stopPropagation(); onPay() }} className="relative z-10 rounded-pill border border-border px-2.5 py-0.5 text-[12px] font-bold hover:bg-muted">Pay</button>
      </span>
    )
  if (e.chequePending) return <MethodBadge method="cheque" status="pending" />
  return e.paidFrom ? <span>{e.paidFrom}</span> : <span className="text-faint">Outside the shop</span>
}
