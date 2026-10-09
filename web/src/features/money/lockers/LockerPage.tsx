import { useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'
import { ArrowLeft, ArrowLeftRight, Printer, SlidersHorizontal } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import type { LockerLedgerRow } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { countText } from '@/features/counter/queries'
import { RangePicker } from '@/features/counter/RangePicker'
import { configQuery, errMsg } from '@/features/till/api'
import { printMoneySlip } from '@/features/till/print'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lockerLedgerQuery, lockerQuery, lockersQuery } from '../queries'
import { AdjustSheet, TransferSheet } from './LockerMoves'
import { kindOf, LEDGER_KINDS } from './kinds'
import { Balance } from './LockersPage'

const route = getRouteApi('/_authed/admin/lockers/$id')
const when = (iso: string) => new Date(iso).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

/** One locker's ledger (karots-pos LockerLedgerPage): every movement in a range,
 * 50 at a time, with the range's net change summed by the server. */
export function LockerPage() {
  const id = Number(route.useParams().id)
  const s = route.useSearch()
  const r = { preset: s.preset ?? 'this-month', from: s.from ?? '', to: s.to ?? '' }
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const v = useQuery(lockerQuery(id, r)).data
  const list = useInfiniteQuery(lockerLedgerQuery(id, r))
  const rows = list.data?.pages.flat() ?? []
  const l = v?.locker
  const others = useQuery(lockersQuery(false)).data?.items ?? []
  const [move, setMove] = useState<'transfer' | 'adjust' | null>(null)
  const net = num(v?.net)
  const reprint = (row: LockerLedgerRow) => void printMoneySlip(row.receiptId).then(() => toast.success(`${row.receiptNo} sent to the printer`), (e) => toast.error(errMsg(e)))

  return (
    <>
      <Link to="/admin/lockers" className="mb-2 inline-flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft size={14} /> Lockers</Link>
      <header className={cn(SURFACE, 'mb-4 flex flex-col gap-4 p-5 md:flex-row md:items-end md:justify-between')}>
        <div className="min-w-0">
          <h1 className="flex flex-wrap items-center gap-2 font-display text-[24px] font-bold tracking-tight md:text-[28px]">
            {l?.name ?? '…'}
            <span className="flex gap-1.5 font-sans tracking-normal">{l && !l.active && <Tag tone="muted">archived</Tag>}{l?.tillDefault && <Tag tone="brand">Till default</Tag>}</span>
          </h1>
          <p className="text-sm text-muted-foreground">{l ? kindOf(l.kind).label : ''}{l?.overdraft ? ' · can go below zero' : ''}{l && !l.cashierAccess ? ' · owner only' : ''}</p>
          <small className="mt-3 block text-[12px] font-semibold text-muted-foreground">In it now</small>
          {l && <Balance sym={sym} v={l.balance} className="font-display text-3xl font-extrabold" />}
        </div>
        <div className="flex flex-col gap-3 md:items-end md:text-right">
          {l?.active && (
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" className="h-10 gap-1.5 px-3.5" onClick={() => setMove('adjust')}><SlidersHorizontal size={15} /> Adjust</Button>
              {others.length >= 2 && l.kind !== 'card' && <Button variant="outline" className="h-10 gap-1.5 px-3.5" onClick={() => setMove('transfer')}><ArrowLeftRight size={15} /> Move money</Button>}
            </div>
          )}
          <div>
            <small className="block text-[12px] font-semibold text-muted-foreground">Net change in range</small>
            <b className={cn('font-display text-2xl tabular-nums', net > 0 && 'text-success', net < 0 && 'text-danger')}>
              {net > 0 ? '↓ ' : net < 0 ? '↑ ' : ''}{money(sym, Math.abs(net))}
            </b>
          </div>
        </div>
      </header>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <RangePicker full value={r} onChange={(x) => void navigate({ search: x, replace: true })} />
        <small className="text-[13px] text-muted-foreground">{countText(rows.length, !!list.hasNextPage)} movement{rows.length === 1 ? '' : 's'}</small>
      </div>
      <DataList label="Ledger" rows={rows} rowKey={(e) => e.id}
        empty={<EmptyState title={list.isPending ? 'Loading…' : 'No movements in this range'} />}
        footer={list.hasNextPage && (
          <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
            className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60">
            {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
          </button>
        )}
        columns={[
          { key: 'what', header: 'Movement', cell: (e) => (
            <span className="flex flex-col">
              <b>{LEDGER_KINDS[e.kind] ?? e.kind}</b>
              <small className="font-normal text-faint">{when(e.at)}{e.note && ` · ${e.note}`}</small>
            </span>
          ) },
          { key: 'receipt', header: 'Receipt', interactive: true, cell: (e) => e.receiptId ? (
            <button type="button" onClick={() => reprint(e)} className="inline-flex items-center gap-1 font-mono text-[12px] text-muted-foreground hover:text-foreground hover:underline">
              <Printer size={13} /> {e.receiptNo}
            </button>
          ) : <span className="text-faint">—</span> },
          { key: 'delta', header: 'Change', num: true, cell: (e) => {
            const d = num(e.delta)
            return <b className={cn('tabular-nums', d > 0 ? 'text-success' : 'text-danger')}>{d > 0 ? '↓ ' : '↑ '}{money(sym, Math.abs(d))}</b>
          } },
        ]} />
      {move === 'transfer' && l && <TransferSheet lockers={others} from={l.id} sym={sym} onClose={() => setMove(null)} />}
      {move === 'adjust' && l && <AdjustSheet l={l} sym={sym} onClose={() => setMove(null)} />}
    </>
  )
}
