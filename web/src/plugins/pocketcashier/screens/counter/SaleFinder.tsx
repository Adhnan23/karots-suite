import { Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '@/api/client'
import type { SaleSummary, SaleView } from '@/api/types.gen'
import { FIELD } from '@/features/till/ui'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD } from '../../bits'
import { findSales, getSale } from '../../store/recent'

/** A sale for a counter page: while the PC answers, as it stands now; otherwise
 * as last synced. Null when the phone has no copy of it. */
export async function loadSale(id: number, live: boolean): Promise<SaleView | null> {
  if (live) {
    try {
      return await api<SaleView>(`/till/sales/${id}`)
    } catch {
      // the PC went away just now: the phone's own copy will do
    }
  }
  return getSale(id)
}

/** Finds a sale from the last 30 days on the phone, and asks the PC for older ones. */
export function SaleFinder({ sym, live, onPick }: { sym: string; live: boolean; onPick: (id: number) => void }) {
  const [q, setQ] = useState('')
  const [rows, setRows] = useState<SaleSummary[] | null>(null)
  useEffect(() => {
    let stale = false
    void (async () => {
      const mine: SaleSummary[] = await findSales(q.trim(), 30).catch(() => [])
      let all = mine
      if (live && q.trim()) {
        const more = await api<SaleSummary[]>(`/till/sales?q=${encodeURIComponent(q.trim())}`).catch(() => [])
        all = [...mine, ...more.filter((m) => !mine.some((s) => s.id === m.id))]
      }
      if (!stale) setRows(all)
    })()
    return () => { stale = true }
  }, [q, live])
  return (
    <>
      <div className="relative">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input aria-label="Find a sale" placeholder="Receipt, customer or product" autoComplete="off" value={q} onChange={(e) => setQ(e.target.value)} className={cn(FIELD, 'h-12 w-full pl-9')} />
      </div>
      {rows?.length === 0 && (
        <p className="m-auto p-8 text-center text-faint">
          {q.trim() && !live ? 'Not in the last 30 days on this phone. Ask them to come back when the PC is on.' : q.trim() ? `No sale matches “${q}”.` : 'No sales yet.'}
        </p>
      )}
      {rows?.map((s) => (
        <button key={s.id} type="button" onClick={() => onPick(s.id)} className={cn(CARD, 'flex min-h-14 items-center gap-3 px-4 py-2.5 text-left')}>
          <span className="min-w-0 flex-1">
            <b className="block truncate text-[15px] tabular-nums">{s.receiptNo}</b>
            <small className="block truncate text-muted-foreground">{new Date(s.createdAt).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })} · {s.customerName || 'Walk-in'}</small>
          </span>
          <b className="shrink-0 tabular-nums">{money(sym, s.total)}</b>
        </button>
      ))}
    </>
  )
}
