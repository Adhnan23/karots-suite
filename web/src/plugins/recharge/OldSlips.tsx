import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { configQuery } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { oldSlipsQuery } from './api'

/** Till Receipts → Reload & Bills (old): the RB- / RL- slips karots-pos printed for its
 * own bill and float forms. New ones are on the sale receipt or a money receipt. */
export function OldSlips() {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [q, setQ] = useState('')
  const dq = useDebounced(q.trim(), 250)
  const rows = useQuery(oldSlipsQuery(dq)).data
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 p-4">
      <input aria-label="Find an old slip" placeholder="RB-000012, bank, reference…" autoComplete="off" value={q} onChange={(e) => setQ(e.target.value)} className={cn(FIELD, 'max-w-sm')} />
      <ul aria-label="Old Reload & Bills slips" className="flex min-h-0 flex-1 flex-col gap-1.5 overflow-auto">
        {rows?.map((r) => (
          <li key={r.no} className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 rounded-[var(--radius-sm)] bg-muted px-3 py-2 text-[13.5px]">
            <b className="truncate">{r.no} · {r.title}</b>
            <b className="text-right tabular-nums">{money(sym, r.amount)}</b>
            <small className="truncate text-muted-foreground">{new Date(r.at).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })} · {r.detail}{r.by && ` · ${r.by}`}</small>
            <small className="text-right text-muted-foreground tabular-nums">{num(r.charge) > 0 ? `+ ${money(sym, r.charge)} charge` : ''}</small>
          </li>
        ))}
        {rows?.length === 0 && <li className="py-8 text-center text-faint">{dq ? `No old slip matches “${dq}”.` : 'No old karots-pos slips.'}</li>}
      </ul>
    </div>
  )
}
