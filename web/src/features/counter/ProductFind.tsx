import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useState } from 'react'
import type { TillProduct } from '@/api/types.gen'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { searchQuery } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { PaneTitle } from './ui'

/** Find a product: type or scan, ↑/↓ to move, Enter to pick. */
export function ProductFind({ title, sym, picked, onPick, filter }: {
  title: string
  sym: string
  picked: number | null
  onPick: (p: TillProduct) => void
  filter?: (p: TillProduct) => boolean
}) {
  const [q, setQ] = useState('')
  const [cur, setCur] = useState(0)
  const res = useQuery({ ...searchQuery(q), placeholderData: keepPreviousData })
  const items = q.trim() ? (res.data?.items ?? []).filter((p) => !filter || filter(p)) : []
  const pick = (p: TillProduct | undefined) => p && onPick(p)
  return (
    <>
      <PaneTitle title={title} />
      <div className="relative mx-3 mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input autoFocus aria-label="Find a product" placeholder="Scan or type a product name" autoComplete="off" value={q}
          onChange={(e) => { setQ(e.target.value); setCur(0) }}
          onKeyDown={(e) => {
            if (e.key === 'ArrowDown') { e.preventDefault(); setCur((i) => Math.min(i + 1, items.length - 1)) }
            else if (e.key === 'ArrowUp') { e.preventDefault(); setCur((i) => Math.max(i - 1, 0)) }
            else if (e.key === 'Enter') { e.preventDefault(); pick(items[cur]) }
          }}
          className={cn(FIELD, 'w-full pl-9')} />
      </div>
      <ul role="listbox" aria-label="Products" className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-2.5 pb-2.5">
        {!q.trim() && <li className="m-auto p-8 text-center text-faint">Scan a barcode or type to find a product.</li>}
        {q.trim() && res.isFetched && items.length === 0 && <li className="m-auto p-8 text-center text-faint">No product matches “{q}”.</li>}
        {items.map((p, i) => (
          <li key={p.id} role="option" aria-selected={picked === p.id}>
            <button type="button" onClick={() => pick(p)} onMouseEnter={() => setCur(i)}
              className={cn('flex w-full items-center gap-3 rounded-[var(--radius-sm)] px-3 py-2 text-left',
                picked === p.id ? 'bg-selected text-selected-foreground' : i === cur ? 'bg-muted' : 'hover:bg-muted')}>
              <span className="min-w-0 flex-1">
                <b className="block truncate text-[14px]">{p.name}</b>
                <small className="opacity-70 tabular-nums">{p.barcode || 'no barcode'} · {money(sym, p.sellingPrice)}</small>
              </span>
              <span className="text-[13px] font-semibold tabular-nums">{qtyText(p.stockQty)} {p.unitAbbr}</span>
            </button>
          </li>
        ))}
      </ul>
    </>
  )
}
