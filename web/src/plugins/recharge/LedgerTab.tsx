import { useQueries, useQuery } from '@tanstack/react-query'
import { Download } from 'lucide-react'
import { useState } from 'react'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { RangePicker } from '@/features/counter/RangePicker'
import { rangeQS, type Range } from '@/features/counter/range'
import { configQuery as tillConfigQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { ledgerCsv, ledgerQuery, reportQuery, setupQuery } from './api'
import { TYPE } from './words'

const SELECT = cn(INPUT, 'h-9 w-auto appearance-auto text-[13px]')

/** Every float row in a range — moves, their commission, counts, float bought — with filters and a CSV. */
export function LedgerTab() {
  const sym = useQuery(tillConfigQuery).data?.symbol ?? 'Rs.'
  const setup = useQuery(setupQuery).data
  const [range, setRange] = useState<Range>({ preset: 'today', from: '', to: '' })
  const [carrier, setCarrier] = useState('')
  const [device, setDevice] = useState('')
  const [type, setType] = useState('')
  const [q, setQ] = useState('')
  const [shown, setShown] = useState(1)
  const dq = useDebounced(q.trim(), 250)
  const qs = `${rangeQS(range).slice(1)}&carrier=${carrier}&device=${device}&type=${type}&q=${encodeURIComponent(dq)}`
  const pages = Array.from({ length: shown }, (_, i) => i)
  const results = useQueries({ queries: pages.map((i) => ledgerQuery(`${qs}&offset=${i * 100}`)) }).map((r) => r.data)
  const rows = results.flatMap((r) => r ?? [])
  const more = (results[results.length - 1]?.length ?? 0) === 100
  return (
    <div className="grid gap-3">
      <RangePicker full noLatest value={range} onChange={(r) => { setRange({ ...r, preset: r.preset || 'today' }); setShown(1) }} />
      <div className="flex flex-wrap items-center gap-1.5">
        <select aria-label="Carrier" value={carrier} onChange={(e) => { setCarrier(e.target.value); setDevice(''); setShown(1) }} className={SELECT}>
          <option value="">All carriers</option>
          {setup?.carriers.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
        <select aria-label="Device" value={device} onChange={(e) => { setDevice(e.target.value); setShown(1) }} className={SELECT}>
          <option value="">All devices</option>
          {setup?.devices.filter((d) => !carrier || d.carrierId === Number(carrier)).map((d) => <option key={d.id} value={d.id}>{d.carrier} · {d.label}</option>)}
        </select>
        <select aria-label="Kind" value={type} onChange={(e) => { setType(e.target.value); setShown(1) }} className={SELECT}>
          <option value="">Everything</option>
          {['reload', 'billpay', 'deposit', 'cashout', 'wallet_in', 'refill', 'commission', 'count', 'reversal'].map((t) => <option key={t} value={t}>{TYPE[t]}</option>)}
        </select>
        <input aria-label="Search the ledger" placeholder="Number, reference, receipt…" value={q} onChange={(e) => { setQ(e.target.value); setShown(1) }} className={cn(INPUT, 'h-9 min-w-0 flex-1 text-[13px]')} />
        <Button asChild variant="secondary" size="sm"><a href={ledgerCsv(qs)} download><Download className="size-4" /> CSV</a></Button>
      </div>
      <DataList label="Float ledger" rows={rows} rowKey={(r) => r.id}
        empty={<EmptyState title="Nothing in this range" />}
        footer={more ? <div className="p-3 text-center"><Button variant="secondary" size="sm" onClick={() => setShown((n) => n + 1)}>Show more</Button></div> : undefined}
        columns={[
          { key: 'w', header: 'What', cell: (r) => <span className="font-semibold">{TYPE[r.type] ?? r.type} · {r.carrier}{r.device && ` · ${r.device}`}</span> },
          { key: 'f', header: 'Float', num: true, cell: (r) => <b className={cn('whitespace-nowrap', num(r.floatDelta) < 0 ? 'text-danger' : num(r.floatDelta) > 0 && 'text-success')}>{num(r.floatDelta) > 0 ? '+' : ''}{money(sym, r.floatDelta)}</b> },
          { key: 'a', header: 'Amount', num: true, wide: true, cell: (r) => <span className="whitespace-nowrap">{money(sym, r.amount)}</span> },
          { key: 'd', header: 'Detail', cell: (r) => [r.detail, r.receipt].filter(Boolean).join(' · ') || '—' },
          { key: 't', header: 'When', cell: (r) => `${new Date(r.at).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })}${r.by ? ` · ${r.by}` : ''}` },
        ]} />
    </div>
  )
}

/** Earnings by carrier and kind, bank bills, and the float counts, for a range. */
export function ReportTab() {
  const sym = useQuery(tillConfigQuery).data?.symbol ?? 'Rs.'
  const [range, setRange] = useState<Range>({ preset: 'this-month', from: '', to: '' })
  const rep = useQuery(reportQuery(rangeQS(range).slice(1))).data
  const m = (s: string) => <span className="whitespace-nowrap">{money(sym, s)}</span>
  return (
    <div className="grid gap-3">
      <RangePicker full noLatest value={range} onChange={(r) => setRange({ ...r, preset: r.preset || 'this-month' })} />
      {rep && (
        <>
          <div className="rounded-[var(--radius)] border border-border bg-card px-3 py-2.5">
            <small className="text-muted-foreground">Earned (commission, count differences{' '}and older service charges)</small>
            <b className={cn('block font-display text-2xl tabular-nums', num(rep.earnings) < 0 ? 'text-danger' : 'text-success')}>{money(sym, rep.earnings)}</b>
            <small className="text-muted-foreground">Service charges are in Sales like any item.</small>
          </div>
          <DataList label="By carrier" rows={rep.lines} rowKey={(r) => `${r.carrier}-${r.kind}`} empty={<EmptyState title="No moves in this range" />}
            columns={[
              { key: 'c', header: 'Carrier · kind', cell: (r) => <span className="font-semibold">{r.carrier} · {TYPE[r.kind] ?? r.kind}</span> },
              { key: 'n', header: 'Moves', num: true, cell: (r) => r.count },
              { key: 'a', header: 'Amount', num: true, cell: (r) => m(r.amount) },
              { key: 'e', header: 'Commission', num: true, cell: (r) => m(r.commission) },
            ]} />
          {rep.banks.length > 0 && (
            <DataList label="Bank accounts" rows={rep.banks} rowKey={(r) => `${r.carrier}-${r.kind}`}
              columns={[
                { key: 'c', header: 'Bank · kind', cell: (r) => <span className="font-semibold">{r.carrier} · {TYPE[r.kind] ?? r.kind}</span> },
                { key: 'n', header: 'Moves', num: true, cell: (r) => r.count },
                { key: 'a', header: 'Amount', num: true, cell: (r) => m(r.amount) },
              ]} />
          )}
          <DataList label="Float counts" rows={rep.counts} rowKey={(r) => `${r.at}-${r.device}`} empty={<EmptyState title="No counts in this range" />}
            columns={[
              { key: 'd', header: 'Device', cell: (r) => <span className="font-semibold">{r.device}</span> },
              { key: 'x', header: 'Expected', num: true, cell: (r) => m(r.expected) },
              { key: 'c', header: 'Counted', num: true, cell: (r) => m(r.counted) },
              { key: 'v', header: 'Difference', num: true, cell: (r) => { const v = num(r.counted) - num(r.expected); return <b className={cn('whitespace-nowrap', v < 0 ? 'text-danger' : v > 0 && 'text-success')}>{v > 0 ? '+' : ''}{money(sym, v)}</b> } },
              { key: 't', header: 'When', cell: (r) => `${new Date(r.at).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })}${r.by ? ` · ${r.by}` : ''}` },
            ]} />
        </>
      )}
    </div>
  )
}
