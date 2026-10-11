import { useState } from 'react'
import { toast } from 'sonner'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { money, num, qtyText, r2 } from '@/lib/money'
import { cn } from '@/lib/utils'
import { approve, snooze, unsnooze } from './api'
import { CHIP } from './ClearancePage'
import type { ClearancePageData, MarkdownRow } from './types'

type Show = 'all' | 'live' | 'snoozed'

/** Candidates and live markdowns: type a %, see the new price, approve or snooze. */
export function MarkdownList({ sym, data }: { sym: string; data?: ClearancePageData }) {
  const [show, setShow] = useState<Show>('all')
  const [pct, setPct] = useState<Record<number, string>>({})
  const [busy, setBusy] = useState<number | null>(null)
  const rows = (data?.rows ?? []).filter((r) => (show === 'all' ? r.status !== 'snoozed' : r.status === show))
  const pctOf = (r: MarkdownRow) => pct[r.productId] ?? r.percent
  const newPrice = (r: MarkdownRow) => r2(num(r.price) * (100 - num(pctOf(r))) / 100)
  const belowFloor = (r: MarkdownRow) => newPrice(r) < num(r.floorCost)
  const run = async (id: number, fn: () => Promise<unknown>, done: string) => {
    setBusy(id)
    try { await fn(); toast.success(done) } catch (e) { toast.error(errMsg(e)) } finally { setBusy(null) }
  }
  const stale = data?.settings.staleDays ?? 60

  return (
    <>
      <div className="mb-3 flex gap-1.5">
        {(['all', 'live', 'snoozed'] as Show[]).map((s) => (
          <button key={s} type="button" aria-pressed={show === s} onClick={() => setShow(s)} className={CHIP}>
            {s === 'all' ? 'All' : s === 'live' ? 'Live' : 'Snoozed'}
          </button>
        ))}
      </div>
      <DataList label="To mark down" rows={rows} rowKey={(r) => r.productId}
        empty={<EmptyState title={!data ? 'Loading…' : show === 'live' ? 'No markdown is running' : show === 'snoozed' ? 'Nothing snoozed' : 'Nothing to mark down'}>
          {data && show === 'all' && `Products show here once they go ${stale} days without a sale, or a batch is about to expire.`}
        </EmptyState>}
        columns={[
          { key: 'name', header: 'Product', cell: (r) => (
            <span className="flex flex-col gap-1">
              <span className="flex flex-wrap items-center gap-1.5">
                <b>{r.name}</b>
                {r.status === 'live' && <Tag tone="brand">{`Live −${r.percent}%`}</Tag>}
                {r.status === 'snoozed' && <Tag tone="muted">{`snoozed to ${r.snoozedUntil}`}</Tag>}
              </span>
              <span className="md:hidden"><Tag tone={r.reason === 'expiring' ? 'warn' : 'muted'}>{r.reasonLabel}</Tag></span>
            </span>
          ) },
          { key: 'why', header: 'Why', wide: true, cell: (r) => <Tag tone={r.reason === 'expiring' ? 'warn' : 'muted'}>{r.reasonLabel}</Tag> },
          { key: 'onHand', header: 'On hand', num: true, cell: (r) => <span className="whitespace-nowrap">{qtyText(r.onHand)} <small className="text-faint">{r.unit}</small></span> },
          { key: 'cost', header: 'Cost', num: true, cell: (r) => (
            <span className="flex flex-col items-end whitespace-nowrap">
              {money(sym, r.cost)}
              {r.reason === 'expiring' && r.floorCost !== r.cost && <small className="text-[11px] text-faint">{`batch ${money(sym, r.floorCost)}`}</small>}
            </span>
          ) },
          { key: 'price', header: 'Price → new', num: true, cell: (r) => (
            <span className="whitespace-nowrap">
              <span className="text-faint line-through">{money(sym, r.price)}</span>{' '}
              <b className={cn(belowFloor(r) ? 'text-danger' : 'text-success')}>{money(sym, newPrice(r))}</b>
            </span>
          ) },
          { key: 'act', header: 'Markdown', interactive: true, className: 'w-[1%]', cell: (r) => (
            <span className="flex items-center justify-end gap-1.5">
              {r.status !== 'snoozed' && <>
                <span className="relative w-[76px]">
                  <input aria-label={`Markdown % for ${r.name}`} inputMode="decimal" value={pctOf(r)} aria-invalid={belowFloor(r)}
                    onChange={(e) => setPct({ ...pct, [r.productId]: e.target.value.replace(/[^\d.]/g, '') })}
                    className={cn(INPUT, 'h-9 pr-6 text-right tabular-nums')} />
                  <span className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-[13px] text-faint">%</span>
                </span>
                <Button className="h-9 w-[84px] font-bold" disabled={busy === r.productId || !(num(pctOf(r)) > 0)}
                  onClick={() => run(r.productId, () => approve(r.productId, pctOf(r)), `${r.name}: −${pctOf(r)}% at the till`)}>
                  {r.status === 'live' ? 'Update' : 'Approve'}
                </Button>
              </>}
              {r.status === 'snoozed'
                ? <Button variant="outline" className="h-9 px-3" disabled={busy === r.productId} onClick={() => run(r.productId, () => unsnooze(r.productId), `${r.name} is back on the list`)}>Bring back</Button>
                : <Button variant="ghost" className="h-9 px-3" disabled={busy === r.productId} onClick={() => run(r.productId, () => snooze(r.productId), `${r.name} snoozed for ${stale} days`)}>Snooze</Button>}
            </span>
          ) },
        ]} />
      {rows.some(belowFloor) && <p className="mt-2 text-[13px] text-danger">A red new price is under cost + the minimum margin — the till will still sell at it if you approve.</p>}
    </>
  )
}
