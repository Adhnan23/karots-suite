import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import type { AdminProductRow } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { PickerShell } from '@/components/admin/PickerShell'
import { configQuery } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { productsQuery } from './queries'

/** Find a stocked product by name or barcode. `trigger` renders the button that
 * opens it; Enter picks the top hit (a scanner types the code then Enter). */
export function ProductPicker({ title, onPick, trigger, exclude }: {
  title: string
  onPick: (p: AdminProductRow) => void
  trigger: (open: () => void) => ReactNode
  exclude?: ReadonlySet<number>
}) {
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const dq = useDebounced(q.trim(), 200)
  const sym = useSuspenseQuery(configQuery).data.symbol
  const rows = (useQuery({ ...productsQuery({ q: dq, category: 0, low: false, disabled: false, page: 1 }), enabled: open, placeholderData: keepPreviousData }).data?.items ?? [])
    .filter((p) => !exclude?.has(p.id))
  const pick = (p: AdminProductRow) => { onPick(p); setOpen(false); setQ('') }
  return (
    <>
      {trigger(() => setOpen(true))}
      <PickerShell open={open} onOpenChange={setOpen} title={title}>
        <div className="relative">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input autoFocus aria-label="Find a product" placeholder="Name or barcode" value={q} onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); if (rows[0] && dq === q.trim()) pick(rows[0]) } }} className={cn(INPUT, 'pl-9')} />
        </div>
        <ul aria-label="Products" className="-mx-1 min-h-0 flex-1 overflow-auto">
          {rows.map((p) => (
            <li key={p.id}>
              <button type="button" onClick={() => pick(p)} className="flex w-full items-center gap-3 rounded-[var(--radius-sm)] px-2.5 py-2 text-left text-sm hover:bg-muted">
                <span className="min-w-0 flex-1">
                  <b className="block truncate">{p.name}</b>
                  <small className="text-faint tabular-nums">{p.barcode || p.categoryName}</small>
                </span>
                <span className="text-right text-[13px] tabular-nums">
                  <span className="block">{money(sym, p.sellingPrice)}</span>
                  <small className="text-faint">{qtyText(p.stockQty)} {p.unitAbbr}</small>
                </span>
              </button>
            </li>
          ))}
          {rows.length === 0 && <li className="p-6 text-center text-sm text-faint">{dq ? `No product matches “${dq}”.` : 'Type to search.'}</li>}
        </ul>
      </PickerShell>
    </>
  )
}
