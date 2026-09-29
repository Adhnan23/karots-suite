import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useState } from 'react'
import type { TillCustomer } from '@/api/types.gen'
import { Seg } from '@/features/till/bits'
import { configQuery, customersQuery } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CounterPage, PaneTitle } from '../ui'
import { CreditForm } from './CreditForm'

type Show = 'all' | 'owe' | 'advance'

/** Customers with a balance either way: collect what they owe, or hand back an advance. */
export function CreditTab() {
  const cfg = useSuspenseQuery(configQuery).data
  const all = useQuery(customersQuery).data ?? []
  const [q, setQ] = useState('')
  const [show, setShow] = useState<Show>('owe')
  const [id, setId] = useState<number | null>(null)
  const needle = q.trim().toLowerCase()
  const rows = all.filter((c) => {
    const b = num(c.outstanding)
    if (b === 0 || (show === 'owe' && b < 0) || (show === 'advance' && b > 0)) return false
    return !needle || c.name.toLowerCase().includes(needle) || c.phone.includes(needle)
  })
  const owed = all.reduce((s, c) => s + Math.max(0, num(c.outstanding)), 0)
  const cur = all.find((c) => c.id === id)
  return (
    <CounterPage flush aside={cur ? <CreditForm key={`${cur.id}-${cur.outstanding}`} c={cur} /> : (
      <p className="m-auto max-w-[260px] p-8 text-center text-faint">Pick a customer to take a payment or refund an advance.</p>
    )}>
      <PaneTitle title="Credit" sub={`${money(cfg.symbol, owed)} owed to the shop`}
        right={<Seg label="Show" value={show} onChange={setShow} options={[['owe', 'Owe'], ['advance', 'Advance'], ['all', 'All']]} />} />
      <div className="relative mx-3 mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input autoFocus aria-label="Find a customer" placeholder="Name or phone" autoComplete="off" value={q}
          onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && rows[0] && setId(rows[0].id)}
          className={cn(FIELD, 'w-full pl-9')} />
      </div>
      <ul aria-label="Customers" className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-2.5 pb-2.5">
        {rows.length === 0 && <li className="m-auto p-8 text-center text-faint">{needle ? `No customer matches “${q}”.` : 'Nobody here — all settled.'}</li>}
        {rows.map((c) => <Row key={c.id} c={c} sym={cfg.symbol} on={id === c.id} onPick={() => setId(c.id)} />)}
      </ul>
    </CounterPage>
  )
}

function Row({ c, sym, on, onPick }: { c: TillCustomer; sym: string; on: boolean; onPick: () => void }) {
  const b = num(c.outstanding)
  const limit = num(c.creditLimit)
  const used = limit > 0 && b > 0 ? Math.min(1, b / limit) : 0
  return (
    <li>
      <button type="button" aria-pressed={on} onClick={onPick}
        className="flex w-full items-center gap-3 rounded-[var(--radius-sm)] px-3 py-2 text-left hover:bg-muted aria-pressed:bg-selected aria-pressed:text-selected-foreground">
        <span className="min-w-0 flex-1">
          <b className="block truncate text-[14px]">{c.name}</b>
          <small className="opacity-70 tabular-nums">{c.phone || 'no phone'}{limit > 0 && ` · limit ${money(sym, limit)}`}</small>
          {used > 0 && (
            <span className="mt-1 block h-1 overflow-hidden rounded-full bg-border">
              <span className={cn('block h-full', used >= 0.9 ? 'bg-danger' : 'bg-warning')} style={{ width: `${used * 100}%` }} />
            </span>
          )}
        </span>
        <span className="text-right">
          <b className={cn('block tabular-nums', !on && (b > 0 ? 'text-warning' : 'text-success'))}>{money(sym, Math.abs(b))}</b>
          <small className="opacity-70">{b > 0 ? 'owes' : 'advance'}</small>
        </span>
      </button>
    </li>
  )
}
