import { useQuery } from '@tanstack/react-query'
import { useBlocker, useSearch } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { errMsg } from '@/features/till/api'
import { useDebounced } from '@/lib/useDebounced'
import { useIsPhone } from '@/lib/useMedia'
import { cn } from '@/lib/utils'
import { fieldsQuery, gridQuery, saveValues } from './api'
import { CHIP } from './FieldSheet'
import { FillRow } from './FillRow'
import { cellKey, type Edits } from './types'

const PAGE_SIZE = 50 // plugins/productplus gridPageSize

/** Fill in: set chosen fields across many products (one field on a phone).
 * ?field=&missing=1 opens it on one field's missing products. */
export function FillIn() {
  const s = useSearch({ strict: false }) as { field?: number; missing?: number }
  const phone = useIsPhone()
  const fields = useQuery(fieldsQuery(false)).data ?? []
  const [picked, setPicked] = useState<number[]>(s.field ? [Number(s.field)] : [])
  const [missing, setMissing] = useState(!!s.missing)
  const [cat, setCat] = useState(0)
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const [dirty, setDirty] = useState<Record<string, string>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)
  const dq = useDebounced(q.trim(), 250)

  const live = picked.filter((id) => fields.some((f) => f.id === id))
  const ids = live.length ? live : fields.slice(0, 1).map((f) => f.id)
  const shown = phone ? ids.slice(0, 1) : ids
  const cols = shown.map((id) => fields.find((f) => f.id === id)!)
  const data = useQuery(gridQuery(shown, cat, missing, dq, page)).data
  const changes = Object.keys(dirty).length
  const blocker = useBlocker({ shouldBlockFn: () => changes > 0, enableBeforeUnload: () => changes > 0, withResolver: true })

  const filter = (f: () => void) => { f(); setPage(1) }
  const pick = (id: number) => filter(() => setPicked(phone ? [id] : !shown.includes(id) ? [...shown, id] : shown.length > 1 ? shown.filter((x) => x !== id) : shown))
  const e: Edits = {
    dirty, errors,
    edit: (pid, fid, v) => {
      const k = cellKey(pid, fid)
      setDirty((d) => ({ ...d, [k]: v }))
      setErrors((x) => {
        const n = { ...x }
        delete n[k]
        return n
      })
    },
  }

  async function save() {
    setBusy(true)
    try {
      const cells = Object.entries(dirty).map(([k, value]) => {
        const [productId, fieldId] = k.split(':').map(Number)
        return { productId, fieldId, value }
      })
      const r = await saveValues(cells)
      const errs = Object.fromEntries(r.errors.map((x) => [cellKey(x.productId, x.fieldId), x.message]))
      setErrors(errs)
      setDirty((d) => Object.fromEntries(Object.entries(d).filter(([k]) => k in errs)))
      if (r.saved) toast.success(`${r.saved} saved`)
      if (r.errors.length) toast.error(`${r.errors.length} not saved — fix the marked cells`)
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  if (!fields.length) return <div className={SURFACE}><EmptyState title="No fields to fill in">Add a field on the Fields tab first.</EmptyState></div>
  const rows = data?.rows ?? []
  return (
    <>
      <div className="mb-3 grid gap-2">
        <div role="group" aria-label="Fields to fill in" className="flex flex-wrap gap-1.5">
          {fields.map((f) => <button key={f.id} type="button" aria-pressed={shown.includes(f.id)} className={CHIP} onClick={() => pick(f.id)}>{f.label}</button>)}
        </div>
        <div className="grid gap-2 md:flex md:items-center">
          <div className="relative md:w-72">
            <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
            <input aria-label="Search products" placeholder="Name or barcode" value={q} onChange={(ev) => filter(() => setQ(ev.target.value))} className={cn(INPUT, 'pl-9')} />
          </div>
          <div className="md:w-64"><CategoryPicker label="Filter by category" noneLabel="All categories" value={cat} onChange={(c) => filter(() => setCat(c))} /></div>
          <button type="button" aria-pressed={missing} className={cn(CHIP, 'justify-self-start')} onClick={() => filter(() => setMissing(!missing))}>Missing only</button>
        </div>
      </div>
      <div className={cn(SURFACE, 'overflow-hidden')}>
        {rows.length === 0 ? (
          <EmptyState title={!data ? 'Loading…' : missing ? 'Nothing missing' : 'Nothing matches'}>{data && missing && 'Every product here has a value for these fields.'}</EmptyState>
        ) : phone ? (
          <ul aria-label="Products to fill in" className="divide-y divide-border">{rows.map((r) => <FillRow key={r.id} card r={r} cols={cols} e={e} />)}</ul>
        ) : (
          <div className="overflow-x-auto">
            <table aria-label="Products to fill in" className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-[12px] font-bold uppercase tracking-wide text-faint">
                  <th className="px-4 py-2.5">Product</th>
                  {cols.map((f) => <th key={f.id} className="px-4 py-2.5">{f.label}{f.required && ' *'}</th>)}
                </tr>
              </thead>
              <tbody>{rows.map((r) => <FillRow key={r.id} r={r} cols={cols} e={e} />)}</tbody>
            </table>
          </div>
        )}
        {data && <Pager page={page} pageSize={PAGE_SIZE} total={data.total} onPage={setPage} />}
      </div>
      {changes > 0 && (
        <div className="sticky bottom-[96px] z-10 mt-3 flex items-center gap-3 rounded-[var(--radius)] border border-border bg-popover p-3 shadow-[var(--shadow-float)] md:bottom-[100px]">
          <span className="flex-1 text-sm"><b>{changes}</b> {changes === 1 ? 'change' : 'changes'}</span>
          <Button variant="ghost" className="h-10 px-3" onClick={() => { setDirty({}); setErrors({}) }}>Discard</Button>
          <Button className="h-10 px-5 font-bold" disabled={busy} onClick={() => void save()}>Save</Button>
        </div>
      )}
      <Dialog open={blocker.status === 'blocked'} onOpenChange={(o) => { if (!o) blocker.reset?.() }}>
        <DialogContent className="gap-3 p-5 sm:max-w-sm">
          <DialogTitle className="font-display text-lg font-bold">Leave without saving?</DialogTitle>
          <DialogDescription>{changes} {changes === 1 ? 'change' : 'changes'} on Fill in will be lost.</DialogDescription>
          <div className="mt-2 flex justify-end gap-2">
            <Button type="button" variant="ghost" className="h-10 px-4" onClick={() => blocker.reset?.()}>Stay</Button>
            <Button type="button" className="h-10 bg-danger px-4 font-bold text-danger-foreground hover:bg-danger/90" onClick={() => blocker.proceed?.()}>Leave</Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
