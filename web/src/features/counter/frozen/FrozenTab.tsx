import { useSuspenseQuery } from '@tanstack/react-query'
import { Search, Snowflake } from 'lucide-react'
import { useState } from 'react'
import type { FrozenProductDTO } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { frozenQuery } from '@/features/freeze/api'
import { ToShelfDialog } from '@/features/freeze/ToShelfDialog'
import { UnfreezeDialog } from '@/features/freeze/UnfreezeDialog'
import { FIELD } from '@/features/till/ui'
import { qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CounterPage, PaneTitle } from '../ui'

/** Stock freeze at the counter: the products whose goods out for sale aren't counted
 * yet. When they run low, unfreeze with how many are left; opening a box is "Put out for sale". */
export function FrozenTab() {
  const data = useSuspenseQuery(frozenQuery).data
  const [q, setQ] = useState('')
  const [id, setId] = useState<number | null>(null)
  const shown = data.products.filter((p) => p.name.toLowerCase().includes(q.trim().toLowerCase()))
  const cur = data.products.find((p) => p.id === id)
  return (
    <CounterPage flush aside={cur ? <Actions key={cur.id} p={cur} onDone={() => setId(null)} /> : (
      <p className="m-auto max-w-[280px] p-8 text-center text-faint">
        {data.products.length ? 'Pick a product on the left to unfreeze it or put a box out for sale.' : 'Nothing is frozen: every sale counts.'}
      </p>
    )}>
      <PaneTitle title="Frozen" sub={`${data.products.length} ${data.products.length === 1 ? 'product' : 'products'}${data.shopFrozen ? ' · new ones start frozen' : ''}`} />
      <div className="relative mx-3 mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input autoFocus aria-label="Find a frozen product" placeholder="Find a product" autoComplete="off" value={q}
          onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && shown[0] && setId(shown[0].id)}
          className={cn(FIELD, 'w-full pl-9')} />
      </div>
      <ul aria-label="Frozen products" className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-2.5 pb-2.5">
        {shown.map((p) => (
          <li key={p.id}>
            <button type="button" aria-pressed={id === p.id} onClick={() => setId(p.id)}
              className="flex w-full items-center gap-2 rounded-[var(--radius-sm)] px-3 py-2 text-left hover:bg-muted aria-pressed:bg-selected aria-pressed:text-selected-foreground">
              <Snowflake size={15} className="shrink-0 opacity-60" />
              <b className="min-w-0 flex-1 truncate">{p.name}</b>
              <small className="shrink-0 tabular-nums opacity-75">{qtyText(p.stock)} {p.unit} counted</small>
            </button>
          </li>
        ))}
      </ul>
    </CounterPage>
  )
}

function Actions({ p, onDone }: { p: FrozenProductDTO; onDone: () => void }) {
  const [open, setOpen] = useState<'' | 'unfreeze' | 'shelf'>('')
  return (
    <div className="flex flex-col gap-4 p-5">
      <div>
        <h2 className="font-display text-xl font-bold">{p.name}</h2>
        <p className="text-[13px] text-muted-foreground tabular-nums">
          {qtyText(p.stock)} {p.unit} counted in storage{p.since && ` · frozen since ${new Date(p.since).toLocaleDateString([], { day: 'numeric', month: 'short' })}`}
        </p>
        <p className="mt-2 text-[13px] text-muted-foreground">Sales don't take it out of stock until it's unfrozen.</p>
      </div>
      <Button size="lg" className="h-12 text-[15px]" onClick={() => setOpen('unfreeze')}>Unfreeze…</Button>
      <Button size="lg" variant="outline" className="h-12 text-[15px]" onClick={() => setOpen('shelf')}>Put out for sale…</Button>
      {open === 'unfreeze' && <UnfreezeDialog id={p.id} name={p.name} unit={p.unit} onDone={onDone} onClose={() => setOpen('')} />}
      {open === 'shelf' && <ToShelfDialog id={p.id} onClose={() => setOpen('')} />}
    </div>
  )
}
