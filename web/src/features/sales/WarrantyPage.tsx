import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { InfoSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Tag } from '@/features/admin/products/ProductsPage'
import { countText, warrantyUnitsQuery } from '@/features/counter/queries'
import { ReceiptClaim } from '@/features/counter/warranty/ReceiptClaim'
import { UnitClaim } from '@/features/counter/warranty/UnitClaim'
import { stamp } from '@/features/money/format'
import { Seg } from '@/features/till/bits'
import { cn } from '@/lib/utils'

const route = getRouteApi('/_authed/admin/warranty')
type Status = 'active' | 'expired' | 'replaced' | 'all'

/** Warranty (karots-pos /admin/warranty): every serial sold with cover — find
 * one, see its history, swap a faulty unit; or claim an un-serialled item by
 * its receipt. The counter's claim panels, in the admin. */
export function WarrantyPage() {
  const s = route.useSearch()
  const status = (s.status ?? 'active') as Status
  const navigate = route.useNavigate()
  const [q, setQ] = useState('')
  const dq = useDeferredValue(q.trim())
  const [serial, setSerial] = useState<string | null>(null)
  const [receipt, setReceipt] = useState('')
  const [claim, setClaim] = useState<string | null>(null)
  const list = useInfiniteQuery({ ...warrantyUnitsQuery(status, dq), placeholderData: keepPreviousData })
  const rows = list.data?.pages.flat() ?? []
  return (
    <>
      <PageHead title="Warranty" sub="Every unit sold with cover. Swap a faulty one, or claim an item without a serial by its receipt." />
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <div className="relative md:w-80">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Serial or product" placeholder="Serial, product or customer" value={q} onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && q.trim() && setSerial(q.trim())} className={cn(INPUT, 'pl-9 font-mono')} />
        </div>
        <Seg label="Status" value={status} onChange={(v) => void navigate({ search: { status: v === 'active' ? undefined : v }, replace: true })}
          options={[['active', 'Covered'], ['expired', 'Expired'], ['replaced', 'Replaced'], ['all', 'All']]} />
        <form className="relative md:ml-auto md:w-72" onSubmit={(e) => { e.preventDefault(); if (receipt.trim()) setClaim(receipt.trim().toUpperCase()) }}>
          <input aria-label="Claim by receipt" placeholder="No serial? Receipt no, then Enter" value={receipt} onChange={(e) => setReceipt(e.target.value)} className={cn(INPUT, 'font-mono')} />
        </form>
      </div>
      <div className="mb-2 text-right text-[13px] text-muted-foreground">{countText(rows.length, !!list.hasNextPage)} units</div>
      <DataList label="Units" rows={rows} rowKey={(u) => u.id} onRow={(u) => setSerial(u.serial)}
        empty={<EmptyState title={list.isPending ? 'Loading…' : 'No units here'} />}
        footer={list.hasNextPage && (
          <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
            className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60">
            {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
          </button>
        )}
        columns={[
          { key: 'p', header: 'Unit', cell: (u) => (
            <span className="flex flex-col"><b>{u.productName}</b><small className="font-mono font-normal text-faint">{u.serial}</small></span>
          ) },
          { key: 'who', header: 'Sold', cell: (u) => (
            <span className="flex flex-col"><span>{u.customerName || 'Walk-in'}</span><small className="text-faint">{stamp(u.soldAt)}{u.receiptNo && ` · ${u.receiptNo}`}</small></span>
          ) },
          { key: 'cover', header: 'Cover', num: true, cell: (u) => (u.status === 'replaced' ? <Tag tone="muted">replaced</Tag>
            : <span className={cn('font-semibold', u.underWarranty ? 'text-success' : 'text-muted-foreground')}>{u.left}</span>) },
        ]} />
      <InfoSheet open={!!serial} onOpenChange={(o) => !o && setSerial(null)} title="Warranty unit">
        {serial && <UnitClaim key={serial} serial={serial} onSerial={setSerial} />}
      </InfoSheet>
      <InfoSheet open={!!claim} onOpenChange={(o) => !o && setClaim(null)} title={`Claim on ${claim ?? ''}`}>
        {claim && <div className="p-5"><ReceiptClaim key={claim} no={claim} /></div>}
      </InfoSheet>
    </>
  )
}
