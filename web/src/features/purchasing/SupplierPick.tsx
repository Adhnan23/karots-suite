import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { INPUT } from '@/components/admin/fields'
import { PickerShell } from '@/components/admin/PickerShell'
import { Button } from '@/components/ui/button'
import { supplierOptionsQuery } from '@/features/admin/products/queries'
import { cn } from '@/lib/utils'
import { SupplierForm } from './suppliers/SupplierForm'

/** Pick a supplier by searching, or add one on the spot and carry on with the form behind it. */
export function SupplierPick({ label, value, onChange }: { label: string; value: number; onChange: (id: number) => void }) {
  const qc = useQueryClient()
  const suppliers = useQuery(supplierOptionsQuery).data ?? []
  const [adding, setAdding] = useState(false)
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const term = q.trim().toLowerCase()
  const rows = suppliers.filter((s) => !term || s.name.toLowerCase().includes(term))
  const name = suppliers.find((s) => s.id === value)?.name
  const pick = (id: number) => { onChange(id); setOpen(false); setQ('') }
  return (
    <div className="flex gap-1.5">
      <button type="button" aria-label={label} onClick={() => setOpen(true)} className={cn(INPUT, 'flex min-w-0 flex-1 items-center justify-between gap-2 text-left')}>
        <span className={cn('truncate', !name && 'text-faint')}>{name ?? 'None'}</span>
        <ChevronDown size={16} className="flex-none text-faint" />
      </button>
      <Button type="button" variant="outline" className="h-[var(--control-h)] gap-1.5 px-3" onClick={() => setAdding(true)}><Plus size={14} /> New</Button>
      <PickerShell open={open} onOpenChange={setOpen} title={label}>
        <div className="relative">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input autoFocus aria-label="Search suppliers" placeholder="Search suppliers" value={q} onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && rows[0] && (e.preventDefault(), pick(rows[0].id))} className={cn(INPUT, 'pl-9')} />
        </div>
        <ul aria-label="Suppliers" className="-mx-1 min-h-0 flex-1 overflow-auto">
          {!term && <li><button type="button" aria-pressed={value === 0} onClick={() => pick(0)} className={ROW}>None</button></li>}
          {rows.map((s) => (
            <li key={s.id}><button type="button" aria-pressed={value === s.id} onClick={() => pick(s.id)} className={ROW}><span className="truncate">{s.name}</span></button></li>
          ))}
          {rows.length === 0 && <li className="p-6 text-center text-sm text-faint">No supplier matches “{q}”.</li>}
        </ul>
        <button type="button" onClick={() => { setOpen(false); setAdding(true) }} className="flex items-center gap-1.5 self-start rounded-pill px-2 py-1 text-[13px] font-bold text-muted-foreground hover:text-foreground">
          <Plus size={15} /> New supplier
        </button>
      </PickerShell>
      {/* The sheet is a portal, but its submit still bubbles to any form this sits in. */}
      {adding && <div onSubmit={(e) => e.stopPropagation()}>
        <SupplierForm onClose={() => setAdding(false)} onSaved={async (r) => {
          await qc.fetchQuery({ ...supplierOptionsQuery, staleTime: 0 })
          onChange(r.id)
        }} />
      </div>}
    </div>
  )
}

const ROW = 'flex w-full min-w-0 items-center gap-2 rounded-[var(--radius-sm)] px-2.5 py-2 text-left text-sm hover:bg-muted aria-pressed:bg-selected aria-pressed:text-selected-foreground'
