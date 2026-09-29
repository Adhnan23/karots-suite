import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, CornerDownRight, Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { CategoryQuickResult } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { PickerShell } from '@/components/admin/PickerShell'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { categoriesQuery } from './queries'
import { pathName, visibleTree } from './tree'

/** Pick a category from the tree. value 0 = none (shown as noneLabel, offered
 * only when noneLabel is set). allowCreate adds a "+" per row that creates a
 * sub-category right there (find-or-create, so a repeat picks the existing one). */
export function CategoryPicker({ value, onChange, label, noneLabel, exclude, allowCreate }: {
  value: number
  onChange: (id: number) => void
  label: string
  noneLabel?: string
  exclude?: ReadonlySet<number>
  allowCreate?: boolean
}) {
  const nodes = useQuery(categoriesQuery).data ?? []
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const [adding, setAdding] = useState<number | null>(null) // parent id (0 = top level) of the inline create
  const [name, setName] = useState('')
  const rows = visibleTree(nodes, q, new Set()).filter((n) => !exclude?.has(n.id))
  const pick = (id: number) => { onChange(id); setOpen(false); setQ(''); setAdding(null) }

  async function create(parentId: number) {
    try {
      const r = await api<CategoryQuickResult>('/admin/categories/quick', { json: { name, parentId } })
      await qc.invalidateQueries({ queryKey: ['admin', 'categories'] })
      setName('')
      pick(r.id)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  const createRow = (parentId: number, depth: number) => (
    <li className="flex gap-1.5 py-1" style={{ paddingLeft: 8 + depth * 18 }}>
      <CornerDownRight size={16} className="mt-3 flex-none text-faint" />
      <input autoFocus aria-label="New category name" placeholder="Name" value={name} onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void create(parentId) } if (e.key === 'Escape') { e.stopPropagation(); setAdding(null) } }}
        className={cn(INPUT, 'h-10 flex-1')} />
      <button type="button" onClick={() => void create(parentId)} className="rounded-[var(--radius-sm)] bg-selected px-3 text-[13px] font-bold text-selected-foreground">Add</button>
    </li>
  )

  return (
    <>
      <button type="button" aria-label={label} onClick={() => setOpen(true)} className={cn(INPUT, 'flex items-center justify-between gap-2 text-left')}>
        <span className={cn('truncate', !value && 'text-faint')}>{value ? pathName(nodes, value) || '…' : (noneLabel ?? 'Pick a category')}</span>
        <ChevronDown size={16} className="flex-none text-faint" />
      </button>
      <PickerShell open={open} onOpenChange={setOpen} title={label}>
        <div className="relative">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input autoFocus aria-label="Search categories" placeholder="Search categories" value={q} onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && rows[0] && (e.preventDefault(), pick(rows[0].id))} className={cn(INPUT, 'pl-9')} />
        </div>
        <ul aria-label="Categories" className="-mx-1 min-h-0 flex-1 overflow-auto">
          {noneLabel && !q && (
            <li><button type="button" aria-pressed={value === 0} onClick={() => pick(0)} className={ROW}>{noneLabel}</button></li>
          )}
          {rows.map((n) => (
            <li key={n.id}>
              <div className="flex items-center">
                <button type="button" aria-pressed={value === n.id} onClick={() => pick(n.id)} className={ROW} style={{ paddingLeft: 10 + n.depth * 18 }}>
                  <span className={cn('truncate', n.depth === 0 && 'font-bold')}>{n.name}</span>
                  {n.productCount > 0 && <small className="ml-auto flex-none opacity-60 tabular-nums">{n.productCount}</small>}
                </button>
                {allowCreate && (
                  <button type="button" aria-label={`Add under ${n.name}`} onClick={() => { setAdding(n.id); setName('') }}
                    className="grid h-9 w-9 flex-none place-items-center rounded-full text-faint hover:bg-muted hover:text-foreground"><Plus size={16} /></button>
                )}
              </div>
              {adding === n.id && <ul>{createRow(n.id, n.depth + 1)}</ul>}
            </li>
          ))}
          {rows.length === 0 && <li className="p-6 text-center text-sm text-faint">No category matches “{q}”.</li>}
        </ul>
        {allowCreate && (adding === 0 ? <ul>{createRow(0, 0)}</ul> : (
          <button type="button" onClick={() => { setAdding(0); setName(q) }} className="flex items-center gap-1.5 self-start rounded-full px-2 py-1 text-[13px] font-bold text-muted-foreground hover:text-foreground">
            <Plus size={15} /> New top-level category
          </button>
        ))}
      </PickerShell>
    </>
  )
}

const ROW = 'flex min-w-0 flex-1 items-center gap-2 rounded-[var(--radius-sm)] px-2.5 py-2 text-left text-sm hover:bg-muted aria-pressed:bg-selected aria-pressed:text-selected-foreground'
