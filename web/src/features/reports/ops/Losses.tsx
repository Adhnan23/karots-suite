import { keepPreviousData, useInfiniteQuery, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useState } from 'react'
import type { LossRow } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { countText } from '@/features/counter/queries'
import { stamp } from '@/features/money/format'
import { configQuery } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { periodText } from '../kit/numbers'
import { ReportPage } from '../kit/ReportPage'
import { rangeOf, reportQS } from '../kit/search'
import { lossLinesQuery, lossesQuery } from '../queries'
import { RecoverSheet } from './RecoverSheet'

const route = getRouteApi('/_authed/admin/reports/losses')
const OUTCOME: Record<string, string> = { replacement: 'replaced by supplier', paid: 'credited by supplier', written_off: 'written off' }

/** Losses & Recovery (Damage and Warranty
 * together): stock lost to damage and warranty swaps, and what suppliers made
 * good. Record a recovery against any loss. */
export function LossesPage() {
  const r = rangeOf(route.useSearch())
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery({ ...lossesQuery(r), placeholderData: keepPreviousData }).data
  const list = useInfiniteQuery({ ...lossLinesQuery(r), placeholderData: keepPreviousData })
  const rows = list.data?.pages.flat() ?? []
  const [recover, setRecover] = useState<LossRow | null>(null)
  return (
    <ReportPage rows={list} title="Losses and recovery" sub="Stock lost to damage and warranty swaps, and what suppliers made good."
      range={r} onRange={(x) => void navigate({ search: (p) => ({ ...p, ...x }), replace: true })} caption={d && periodText(d.from, d.to)}
      exportUrl={`/api/v1/admin/reports/losses?${reportQS(r)}`}>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label="Damaged" amount={d?.damageCost} tone="out" v={d ? money(sym, d.damageCost) : '…'} sub={d && `${d.damageCount} write-off${d.damageCount === 1 ? '' : 's'}`} />
        <Kpi label="Warranty swaps" amount={d?.warrantyCost} tone="out" v={d ? money(sym, d.warrantyCost) : '…'} sub={d && `${d.warrantyCount} unit${d.warrantyCount === 1 ? '' : 's'}`} />
        <Kpi label="↓ Made good by suppliers" amount={d?.recovered} tone="in" v={d ? money(sym, d.recovered) : '…'} sub="booked in this period" />
        <Kpi label="Net loss" amount={d?.net} tone="out" v={d ? money(sym, d.net) : '…'} />
      </KpiStrip>
      <div>
        <div className="mb-2 text-right text-[13px] text-muted-foreground">{countText(rows.length, !!list.hasNextPage)} losses</div>
        <DataList label="Losses" rows={rows} rowKey={(x) => `${x.kind}-${x.id}`}
          empty={<EmptyState title={list.isPending ? 'Loading…' : 'Nothing lost in this period'} />}
          footer={list.hasNextPage && (
            <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
              className="w-full border-t border-border px-4 py-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60 print:hidden">
              {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
            </button>
          )}
          columns={[
            { key: 'p', header: 'Loss', cell: (x) => (
              <span className="flex flex-col">
                <span className="flex items-center gap-1.5"><b>{x.product}</b><Tag tone={x.kind === 'damage' ? 'warn' : 'muted'}>{x.kind === 'damage' ? 'damage' : 'warranty'}</Tag></span>
                <small className="font-normal text-faint">{stamp(x.at)} · {qtyText(x.qty)} · {x.by}{x.note && ` · ${x.note}`}</small>
              </span>
            ) },
            { key: 'loss', header: 'Lost', num: true, cell: (x) => <b>{money(sym, x.loss)}</b> },
            { key: 'rec', header: 'Recovery', num: true, cell: (x) => (x.outcome
              ? <span className="flex flex-col items-end"><span className="text-success">{OUTCOME[x.outcome] ?? x.outcome}</span>{x.outcome !== 'written_off' && <small className="text-faint">{money(sym, x.recovered)}</small>}</span>
              : x.sourceId
                ? <Button variant="outline" className="h-8 px-3 text-[12px] print:hidden" onClick={() => setRecover(x)}>Recover</Button>
                : <small className="text-faint">can’t be traced</small>) },
          ]} />
      </div>
      {recover && <RecoverSheet loss={recover} sym={sym} onClose={() => setRecover(null)} />}
    </ReportPage>
  )
}
