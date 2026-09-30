import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { Plus, Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { AdminSupplierRow } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { suppliersQuery } from '../queries'
import { SupplierForm } from './SupplierForm'

const route = getRouteApi('/_authed/admin/suppliers/')

/** What a supplier balance means, in words and colour: we owe them, or they owe us. */
export function Owed({ sym, owed, big }: { sym: string; owed: string; big?: boolean }) {
  const n = num(owed)
  if (n === 0) return <span className="text-faint">settled</span>
  return (
    <span className={cn('tabular-nums', n < 0 && 'text-success', big && 'font-display text-3xl font-extrabold')}>
      {money(sym, Math.abs(n))}{n < 0 && <small className="ml-1 text-[11px] font-bold">cr</small>}
    </span>
  )
}

/** Suppliers (karots-pos /admin/suppliers): who we buy from and what's owed either way. */
export function SuppliersPage() {
  const s = route.useSearch()
  const f = { q: s.q ?? '', show: s.show ?? '', disabled: !!s.disabled, page: s.page ?? 1 }
  const navigate = route.useNavigate()
  const go = useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [q, setQ] = useState(f.q)
  const dq = useDebounced(q, 250)
  useEffect(() => { if (dq !== f.q) void navigate({ search: (x) => ({ ...x, q: dq, page: 1 }), replace: true }) }, [dq, f.q, navigate])
  const data = useQuery({ ...suppliersQuery(f), placeholderData: keepPreviousData }).data
  const [adding, setAdding] = useState(false)
  const set = (patch: Partial<typeof f>) => void navigate({ search: (x) => ({ ...x, page: 1, ...patch }) })
  const chip = 'rounded-full border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'
  const open = (r: AdminSupplierRow) => void go({ to: '/admin/suppliers/$id', params: { id: String(r.id) } })

  return (
    <>
      <PageHead title="Suppliers" sub="Who you buy from, and what's owed either way."
        actions={<Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setAdding(true)}><Plus size={16} /> New supplier</Button>} />
      {data && (
        <div className="mb-3 grid grid-cols-2 gap-2 md:max-w-md">
          <button type="button" onClick={() => set({ show: f.show === 'owing' ? '' : 'owing' })} className={cn(SURFACE, 'flex flex-col items-start px-4 py-3 text-left', f.show === 'owing' && 'border-selected')}>
            <small className="text-[12px] font-semibold text-muted-foreground">↑ We owe</small>
            <b className="font-display text-lg whitespace-nowrap tabular-nums text-danger md:text-xl">{money(sym, data.owed)}</b>
          </button>
          <button type="button" onClick={() => set({ show: f.show === 'credit' ? '' : 'credit' })} className={cn(SURFACE, 'flex flex-col items-start px-4 py-3 text-left', f.show === 'credit' && 'border-selected')}>
            <small className="text-[12px] font-semibold text-muted-foreground">↓ They owe us</small>
            <b className="font-display text-lg whitespace-nowrap tabular-nums text-success md:text-xl">{money(sym, data.credit)}</b>
          </button>
        </div>
      )}
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <div className="relative md:w-80">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search suppliers" placeholder="Name, contact or phone" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        <div className="flex flex-wrap gap-1.5">
          <button type="button" aria-pressed={f.show === 'owing'} onClick={() => set({ show: f.show === 'owing' ? '' : 'owing' })} className={chip}>We owe</button>
          <button type="button" aria-pressed={f.show === 'credit'} onClick={() => set({ show: f.show === 'credit' ? '' : 'credit' })} className={chip}>In credit</button>
          <button type="button" aria-pressed={f.disabled} onClick={() => set({ disabled: !f.disabled })} className={chip}>Show disabled</button>
        </div>
      </div>
      <DataList label="Suppliers" rows={data?.items ?? []} rowKey={(r) => r.id} onRow={open}
        empty={<EmptyState title={!data ? 'Loading…' : f.q || f.show ? 'Nothing matches' : 'No suppliers yet'}>
          {data && !f.q && !f.show && 'Add the people you buy from, then record deliveries and payments against them.'}
        </EmptyState>}
        footer={data && <Pager page={data.page} pageSize={data.pageSize} total={data.total} onPage={(page) => void navigate({ search: (x) => ({ ...x, page }) })} />}
        columns={[
          { key: 'name', header: 'Supplier', cell: (r) => (
            <span className="flex flex-col gap-0.5">
              <span className="flex flex-wrap items-center gap-1.5">
                <b className={cn(!r.active && 'text-faint line-through')}>{r.name}</b>
                {!r.active && <Tag tone="muted">disabled</Tag>}
                {num(r.oldDebt) > 0 && <Tag tone="warn">old debt</Tag>}
              </span>
              <small className="font-normal text-faint">{[r.contact, r.phone].filter(Boolean).join(' · ') || 'no contact'}</small>
            </span>
          ) },
          { key: 'terms', header: 'Credit', wide: true, cell: (r) => <span className="text-muted-foreground">{r.creditDays ? `${r.creditDays} days` : 'cash'}</span> },
          { key: 'owed', header: 'Balance', num: true, cell: (r) => <Owed sym={sym} owed={r.owed} /> },
        ]} />
      {adding && <SupplierForm onClose={() => setAdding(false)} onSaved={(r) => open(r)} />}
    </>
  )
}
