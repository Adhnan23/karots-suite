import { keepPreviousData, useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '@/api/client'
import type { MoneyReceiptRow } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Tag } from '@/features/admin/products/ProductsPage'
import { countText, debtReceiptsQuery, moneyReceiptsQuery, salesQuery, warrantyClaimsQuery } from '@/features/counter/queries'
import { RangePicker } from '@/features/counter/RangePicker'
import { configQuery } from '@/features/till/api'
import { money } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'
import { stamp } from '../format'
import { type Pick, ReceiptSheet } from './ReceiptSheet'

const route = getRouteApi('/_authed/admin/receipts')

const MOVE_KINDS: [string, string][] = [
  ['', 'Every kind'], ['expense', 'Expenses'], ['expense_void', 'Expenses voided'], ['supplier_payment', 'Supplier payments'], ['supplier_refund', 'Supplier refunds'],
  ['customer_payment', 'Credit collected'], ['refund', 'Refunds'], ['transfer', 'Transfers'], ['bank_charge', 'Bank charges'], ['interest', 'Interest'], ['adjust', 'Adjustments'],
]
const TABS: [string, string][] = [['sales', 'Sales'], ['cash', 'Cash'], ['credit', 'Credit'], ['warranty', 'Warranty'], ...slots.tillReceiptKinds.map((k): [string, string] => [`x:${k.key}`, k.label])]

type Row = { key: number; title: string; sub: string; who: string; amount: string | null; tag?: string; p: Pick }

/** Receipts (karots-pos /admin/receipts): every slip the shop has given — sales
 * (S-), cash moves (CR-), credit payments (DP-) and warranty claims — found,
 * seen as paper and printed again. `cashId` opens one CR- receipt (deep link). */
export function ReceiptsPage({ cashId }: { cashId?: number }) {
  const s = route.useSearch()
  const tab = cashId ? 'cash' : (s.tab ?? 'sales')
  const r = { preset: s.preset ?? '', from: s.from ?? '', to: s.to ?? '' }
  const kind = s.kind ?? ''
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof s>) => void navigate({ to: '/admin/receipts', search: (x) => ({ ...x, ...p }), replace: true })
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [q, setQ] = useState(s.q ?? '')
  const dq = useDebounced(q.trim(), 250)
  useEffect(() => { if (dq !== (s.q ?? '')) set({ q: dq }) }, [dq]) // eslint-disable-line react-hooks/exhaustive-deps
  const [pick, setPick] = useState<Pick | null>(null)
  const deep = useQuery({ queryKey: ['admin', 'money-receipt', cashId], queryFn: () => api<MoneyReceiptRow>(`/admin/money-receipts/${cashId}`), enabled: !!cashId }).data
  const open: Pick | null = pick ?? (deep ? { kind: 'cash', row: deep } : null)

  const opts = { placeholderData: keepPreviousData }
  const salesQ = useInfiniteQuery({ ...salesQuery(dq, r), ...opts, enabled: tab === 'sales' })
  const cashQ = useInfiniteQuery({ ...moneyReceiptsQuery(dq, kind, r), ...opts, enabled: tab === 'cash' })
  const creditQ = useInfiniteQuery({ ...debtReceiptsQuery(dq, r), ...opts, enabled: tab === 'credit' })
  const claimsQ = useInfiniteQuery({ ...warrantyClaimsQuery(dq, r), ...opts, enabled: tab === 'warranty' })
  const list = { sales: salesQ, cash: cashQ, credit: creditQ, warranty: claimsQ }[tab as 'sales'] ?? salesQ
  const rows: Row[] = tab === 'sales'
    ? (salesQ.data?.pages.flat() ?? []).map((x) => ({ key: x.id, title: x.receiptNo, sub: `${stamp(x.createdAt)}${x.customerName ? ` · ${x.customerName}` : ''}`, who: x.cashierName, amount: x.total,
      tag: x.status === 'returned' ? 'returned' : x.status === 'partially_returned' ? 'part returned' : undefined, p: { kind: 'sales', id: x.id } }))
    : tab === 'cash'
      ? (cashQ.data?.pages.flat() ?? []).map((x) => ({ key: x.id, title: x.receiptNo, sub: `${stamp(x.createdAt)} · ${x.kindLabel}${x.party ? ` · ${x.party}` : ''}`, who: `${x.fromLabel} → ${x.toLabel}`, amount: x.amount, p: { kind: 'cash', row: x } }))
      : tab === 'credit'
        ? (creditQ.data?.pages.flat() ?? []).map((x) => ({ key: x.id, title: x.receiptNo, sub: `${stamp(x.createdAt)} · ${x.customerName} · ${x.method}`, who: x.cashierName, amount: x.amount, p: { kind: 'credit', row: x } }))
        : (claimsQ.data?.pages.flat() ?? []).map((x) => ({ key: x.id, title: x.productName, sub: `${stamp(x.at)} · ${x.oldSerial || 'by receipt'}${x.customerName ? ` · ${x.customerName}` : ''}`, who: x.handledBy, amount: null,
          tag: x.resolution, p: { kind: 'warranty', row: x } }))
  const PluginPage = tab.startsWith('x:') ? slots.tillReceiptKinds.find((k) => `x:${k.key}` === tab)?.Page : undefined
  const chip = 'rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

  return (
    <>
      <PageHead title="Receipts" sub="Every slip the shop has given — find it, see it, print it again." />
      <div role="group" aria-label="Receipt kind" className="-mx-1 mb-3 flex gap-1.5 overflow-x-auto px-1 pb-1">
        {TABS.map(([v, l]) => <button key={v} type="button" aria-pressed={tab === v} onClick={() => set({ tab: v, kind: '' })} className={cn(chip, 'whitespace-nowrap')}>{l}</button>)}
      </div>
      {PluginPage ? <PluginPage /> : (
        <>
          <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
            <div className="relative md:w-72">
              <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
              <input aria-label="Find a receipt" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')}
                placeholder={tab === 'sales' ? 'Receipt no, customer, product' : tab === 'cash' ? 'CR- number, party, locker' : tab === 'credit' ? 'Receipt no, customer' : 'Serial or customer'} />
            </div>
            <RangePicker full value={r} onChange={(x) => set(x)} />
            {tab === 'cash' && (
              <select aria-label="Kind of move" value={kind} onChange={(e) => set({ kind: e.target.value })} className={cn(INPUT, 'appearance-auto md:w-56')}>
                {MOVE_KINDS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
              </select>
            )}
            <small className="text-[13px] text-muted-foreground md:ml-auto">{countText(rows.length, !!list.hasNextPage)} shown</small>
          </div>
          <DataList label="Receipts" rows={rows} rowKey={(x) => x.key} onRow={(x) => setPick(x.p)}
            empty={<EmptyState title={list.isPending ? 'Loading…' : 'Nothing found'} />}
            footer={list.hasNextPage && (
              <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
                className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60">
                {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
              </button>
            )}
            columns={[
              { key: 'what', header: tab === 'warranty' ? 'Claim' : 'Receipt', cell: (x) => (
                <span className="flex flex-col">
                  <span className="flex items-center gap-1.5"><b className={cn(tab !== 'warranty' && 'tabular-nums')}>{x.title}</b>{x.tag && <Tag tone="muted">{x.tag}</Tag>}</span>
                  <small className="font-normal text-faint">{x.sub}</small>
                </span>
              ) },
              { key: 'who', header: tab === 'cash' ? 'From → to' : 'By', wide: tab !== 'cash', cell: (x) => <span className="text-muted-foreground">{x.who}</span> },
              { key: 'amount', header: 'Amount', num: true, cell: (x) => x.amount ? <b className="tabular-nums">{money(sym, x.amount)}</b> : <span className="text-faint">—</span> },
            ]} />
        </>
      )}
      {open && <ReceiptSheet pick={open} onClose={() => { setPick(null); if (cashId) set({ tab: 'cash' }) }} />}
    </>
  )
}
