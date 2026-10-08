import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { Plus, Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { AdminCustomerRow } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { SpreadsheetMenu } from '@/components/admin/ImportSheet'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { customersQuery } from '../queries'
import { CustomerForm } from './CustomerForm'

const route = getRouteApi('/_authed/admin/customers/')

/** A customer balance: what they owe us, or the advance they hold (green). */
export function Due({ sym, owed, big }: { sym: string; owed: string; big?: boolean }) {
  const n = num(owed)
  if (n === 0) return <span className="text-faint">settled</span>
  return (
    <span className={cn('tabular-nums', n < 0 && 'text-success', big && 'font-display text-3xl font-extrabold')}>
      {money(sym, Math.abs(n))}{n < 0 && <small className="ml-1 text-[11px] font-bold">adv</small>}
    </span>
  )
}

/** Customers (karots-pos /admin/customers): credit accounts, limits and what's due. */
export function CustomersPage() {
  const s = route.useSearch()
  const f = { q: s.q ?? '', show: s.show ?? '', disabled: !!s.disabled, page: s.page ?? 1 }
  const navigate = route.useNavigate()
  const go = useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q, 250)
  useEffect(() => { if (dq !== f.q) void navigate({ search: (x) => ({ ...x, q: dq, page: 1 }), replace: true }) }, [dq, f.q, navigate])
  const data = useQuery({ ...customersQuery(f), placeholderData: keepPreviousData }).data
  const [adding, setAdding] = useState(false)
  const set = (patch: Partial<typeof f>) => void navigate({ search: (x) => ({ ...x, page: 1, ...patch }) })
  const flip = (show: typeof f.show) => set({ show: f.show === show ? '' : show })
  const chip = 'rounded-full border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'
  const open = (r: AdminCustomerRow) => void go({ to: '/admin/customers/$id', params: { id: String(r.id) } })

  return (
    <>
      <PageHead title="Customers" sub="Credit accounts, limits and what's due."
        actions={<><SpreadsheetMenu entity="customers" /><Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setAdding(true)}><Plus size={16} /> New customer</Button></>} />
      {data && (
        <div className="mb-3 grid grid-cols-2 gap-2 md:max-w-md">
          <button type="button" onClick={() => flip('owing')} className={cn(SURFACE, 'flex flex-col items-start px-4 py-3 text-left', f.show === 'owing' && 'border-selected')}>
            <small className="text-[12px] font-semibold text-muted-foreground">↓ Due to us</small>
            <b className="font-display text-lg whitespace-nowrap tabular-nums md:text-xl">{money(sym, data.owed)}</b>
          </button>
          <button type="button" onClick={() => flip('advance')} className={cn(SURFACE, 'flex flex-col items-start px-4 py-3 text-left', f.show === 'advance' && 'border-selected')}>
            <small className="text-[12px] font-semibold text-muted-foreground">Advances held</small>
            <b className="font-display text-lg whitespace-nowrap tabular-nums text-success md:text-xl">{money(sym, data.advance)}</b>
          </button>
        </div>
      )}
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <div className="relative md:w-80">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search customers" placeholder="Name or phone" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        <div className="flex flex-wrap gap-1.5">
          <button type="button" aria-pressed={f.show === 'owing'} onClick={() => flip('owing')} className={chip}>Owing</button>
          <button type="button" aria-pressed={f.show === 'over'} onClick={() => flip('over')} className={chip}>Over limit</button>
          <button type="button" aria-pressed={f.show === 'advance'} onClick={() => flip('advance')} className={chip}>Advance</button>
          <button type="button" aria-pressed={f.disabled} onClick={() => set({ disabled: !f.disabled })} className={chip}>Show disabled</button>
        </div>
      </div>
      <DataList label="Customers" rows={data?.items ?? []} rowKey={(r) => r.id} onRow={open}
        empty={<EmptyState title={!data ? 'Loading…' : f.q || f.show ? 'Nothing matches' : 'No customers yet'}>
          {data && !f.q && !f.show && 'Customers are added here or at the till when a sale goes on credit.'}
        </EmptyState>}
        footer={data && <Pager page={data.page} pageSize={data.pageSize} total={data.total} onPage={(page) => void navigate({ search: (x) => ({ ...x, page }) })} />}
        columns={[
          { key: 'name', header: 'Customer', cell: (r) => (
            <span className="flex flex-col gap-0.5">
              <span className="flex flex-wrap items-center gap-1.5">
                <b className={cn(!r.active && 'text-faint line-through')}>{r.name}</b>
                {!r.active && <Tag tone="muted">disabled</Tag>}
                {r.overLimit && <Tag tone="danger">over limit</Tag>}
              </span>
              <small className="font-normal text-faint tabular-nums">{r.phone}</small>
            </span>
          ) },
          { key: 'limit', header: 'Limit', num: true, wide: true, cell: (r) => <span className="text-muted-foreground">{num(r.creditLimit) ? money(sym, r.creditLimit) : 'no credit'}</span> },
          { key: 'pts', header: 'Points', num: true, wide: true, cell: (r) => <span className="text-muted-foreground">{r.loyalty || '—'}</span> },
          { key: 'owed', header: 'Balance', num: true, cell: (r) => <Due sym={sym} owed={r.owed} /> },
        ]} />
      {adding && <CustomerForm onClose={() => setAdding(false)} onSaved={open} />}
    </>
  )
}
