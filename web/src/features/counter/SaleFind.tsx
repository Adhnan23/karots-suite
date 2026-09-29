import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { FIELD } from '@/features/till/ui'
import { salesQuery } from './queries'
import { LATEST, type Range } from './range'
import { RangePicker } from './RangePicker'
import { PaneTitle } from './ui'

const STATUS: Record<string, string> = { partially_returned: 'part returned', returned: 'returned', voided: 'voided' }
const when = (iso: string) => {
  const d = new Date(iso)
  const today = d.toDateString() === new Date().toDateString()
  return today ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : d.toLocaleDateString([], { day: 'numeric', month: 'short' })
}

/** Find a sale by receipt no, customer name or phone; latest first. Enter picks the top hit. */
export function SaleFind({ title, sym, picked, onPick }: { title: string; sym: string; picked: number | null; onPick: (id: number) => void }) {
  const [q, setQ] = useState('')
  const [range, setRange] = useState<Range>(LATEST)
  const dq = useDeferredValue(q.trim())
  const rows = useQuery({ ...salesQuery(dq, range), placeholderData: keepPreviousData }).data ?? []
  const qc = useQueryClient()
  // Enter must use the results for exactly what's typed: a scanner types the
  // receipt no and hits Enter before the list catches up, and picking the stale
  // top row would open the wrong sale.
  const pickTop = async () => {
    const hits = await qc.query(salesQuery(q.trim(), range))
    if (hits[0]) onPick(hits[0].id)
  }
  return (
    <>
      <PaneTitle title={title} sub={dq || range.preset ? `${rows.length} found` : 'latest sales'} right={<RangePicker value={range} onChange={setRange} />} />
      <div className="relative mx-3 mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input autoFocus aria-label="Find a sale" placeholder="Receipt no, customer name or phone" autoComplete="off" value={q}
          onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && void pickTop()}
          className={cn(FIELD, 'w-full pl-9')} />
      </div>
      <ul aria-label="Sales" className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-2.5 pb-2.5">
        {rows.length === 0 && <li className="m-auto p-8 text-center text-faint">{dq ? `No sale matches “${dq}”.` : 'No sales yet.'}</li>}
        {rows.map((s) => (
          <li key={s.id}>
            <button type="button" aria-pressed={picked === s.id} onClick={() => onPick(s.id)}
              className="flex w-full items-center gap-3 rounded-[var(--radius-sm)] px-3 py-2 text-left hover:bg-muted aria-pressed:bg-selected aria-pressed:text-selected-foreground">
              <span className="min-w-0 flex-1">
                <b className="text-[14px] tabular-nums">{s.receiptNo}</b>
                {STATUS[s.status] && <small className="ml-2 rounded-full bg-muted px-2 py-0.5 text-[11px] font-semibold text-muted-foreground">{STATUS[s.status]}</small>}
                <small className="block truncate opacity-70">{when(s.createdAt)} · {s.customerName || 'Walk-in'} · {s.cashierName}</small>
              </span>
              <b className="tabular-nums">{money(sym, s.total)}</b>
            </button>
          </li>
        ))}
      </ul>
    </>
  )
}
