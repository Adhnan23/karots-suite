import { ChevronDown, Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { INPUT } from '@/components/admin/fields'
import { PickerShell } from '@/components/admin/PickerShell'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { createGroup } from './api'
import type { Group } from './types'

const ROW = 'flex w-full min-w-0 items-center gap-2 rounded-[var(--radius-sm)] px-2.5 py-2 text-left text-sm hover:bg-muted aria-pressed:bg-selected aria-pressed:text-selected-foreground'

/** Pick an alternatives group like the category picker: search by name,
 * None, or + New group right there — a plain group, no tiers (tiers are set up on the group's page).
 * onPick gets the group's first tier (0 = None). */
export function GroupPicker({ groups, value, onPick }: { groups: Group[]; value: number; onPick: (tierId: number) => void }) {
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const [adding, setAdding] = useState(false)
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const cur = groups.find((g) => g.id === value)
  const t = q.trim().toLowerCase()
  const rows = t ? groups.filter((g) => g.name.toLowerCase().includes(t)) : groups
  const firstTier = (g: Group) => (g.tiers.find((x) => x.active) ?? g.tiers[0])?.id ?? 0
  const done = (tierId: number) => { onPick(tierId); setOpen(false); setQ(''); setAdding(false) }

  async function create() {
    if (!name.trim()) return
    setBusy(true)
    try {
      const g = await createGroup(name.trim())
      setName('')
      done(firstTier(g))
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <button type="button" aria-label="Alternatives group" onClick={() => setOpen(true)} className={cn(INPUT, 'flex items-center justify-between gap-2 text-left')}>
        <span className={cn('truncate', !cur && 'text-faint')}>{cur ? `${cur.name}${cur.active ? '' : ' (off)'}` : 'None'}</span>
        <ChevronDown size={16} className="flex-none text-faint" />
      </button>
      <PickerShell open={open} onOpenChange={setOpen} title="Alternatives group">
        <div className="relative">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input autoFocus aria-label="Search groups" placeholder="Search groups" value={q} onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && rows[0] && (e.preventDefault(), done(firstTier(rows[0])))} className={cn(INPUT, 'pl-9')} />
        </div>
        <ul aria-label="Groups" className="-mx-1 min-h-0 flex-1 overflow-auto">
          {!t && <li><button type="button" aria-pressed={!cur} onClick={() => done(0)} className={ROW}>None</button></li>}
          {rows.map((g) => (
            <li key={g.id}>
              <button type="button" aria-pressed={cur?.id === g.id} onClick={() => done(firstTier(g))} className={ROW}>
                <span className={cn('truncate font-semibold', !g.active && 'opacity-60')}>{g.name}{g.active ? '' : ' (off)'}</span>
                <small className="ml-auto flex-none opacity-60 tabular-nums">{g.products}</small>
              </button>
            </li>
          ))}
          {rows.length === 0 && <li className="p-6 text-center text-sm text-faint">No group matches “{q}”.</li>}
        </ul>
        {adding ? (
          <div className="flex gap-1.5">
            <input autoFocus aria-label="New group name" placeholder="Group name, e.g. 32GB pendrives" value={name} onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void create() } if (e.key === 'Escape') { e.stopPropagation(); setAdding(false) } }}
              className={cn(INPUT, 'h-10 flex-1')} maxLength={80} />
            <button type="button" disabled={busy || !name.trim()} onClick={() => void create()}
              className="rounded-[var(--radius-sm)] bg-selected px-3 text-[13px] font-bold text-selected-foreground disabled:opacity-50">Add</button>
          </div>
        ) : (
          <button type="button" onClick={() => { setAdding(true); setName(q) }} className="flex items-center gap-1.5 self-start rounded-full px-2 py-1 text-[13px] font-bold text-muted-foreground hover:text-foreground">
            <Plus size={15} /> New group
          </button>
        )}
      </PickerShell>
    </>
  )
}
