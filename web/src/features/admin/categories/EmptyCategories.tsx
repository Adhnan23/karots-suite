import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { DeleteEmptyCategoriesResult, EmptyCategory } from '@/api/types.gen'
import { FormSheet } from '@/components/admin/FormSheet'
import { errMsg } from '@/features/till/api'

/** Delete empty categories (docs/rules/stock.md CAT-1, 2): every category with no
 *  products and no sub-categories, ticked; untick the ones to keep (one made for
 *  products still to come). The server checks each again before deleting it. */
export function EmptyCategories({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient()
  const list = useQuery({ queryKey: ['admin', 'categories', 'empty'], queryFn: () => api<EmptyCategory[]>('/admin/categories/empty') }).data
  const [off, setOff] = useState<Set<number>>(new Set())
  const [busy, setBusy] = useState(false)
  const ticked = (list ?? []).filter((c) => !off.has(c.id))
  const flip = (id: number) => setOff((s) => { const n = new Set(s); if (!n.delete(id)) n.add(id); return n })

  const run = async () => {
    if (ticked.length === 0) return onClose()
    setBusy(true)
    try {
      const r = await api<DeleteEmptyCategoriesResult>('/admin/categories/delete-empty', { method: 'POST', json: { ids: ticked.map((c) => c.id) } })
      toast.success(`${r.deleted} empty ${r.deleted === 1 ? 'category' : 'categories'} deleted`)
      if (r.kept.length > 0) toast.info(`Kept: ${r.kept.join('; ')}`)
      void qc.invalidateQueries({ queryKey: ['admin', 'categories'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Delete empty categories" busy={busy}
      submit={ticked.length ? `Delete ${ticked.length}` : 'Close'} onSubmit={() => void run()}>
      <p className="text-[13px] text-muted-foreground">These have no products and no sub-categories. <b className="text-foreground">Untick any you want to keep</b> — say one made for products you'll add later. A parent left empty shows up here next time.</p>
      {!list ? <p className="text-sm text-muted-foreground">Loading…</p> : list.length === 0 ? <p className="text-sm">No empty categories.</p> : (
        <>
          <div className="flex gap-3 text-[13px]">
            <button type="button" className="font-semibold text-brand" onClick={() => setOff(new Set())}>Tick all</button>
            <button type="button" className="font-semibold text-brand" onClick={() => setOff(new Set(list.map((c) => c.id)))}>Untick all</button>
          </div>
          <ul className="grid max-h-[55vh] min-w-0 gap-1 overflow-y-auto">
            {list.map((c) => (
              <li key={c.id}>
                <label className="flex min-w-0 items-start gap-2 rounded-[calc(var(--radius)-4px)] px-2 py-1.5 text-sm hover:bg-muted/50">
                  <input type="checkbox" className="mt-0.5 size-4 shrink-0 accent-[var(--brand)]" checked={!off.has(c.id)} onChange={() => flip(c.id)} />
                  <span className="min-w-0 break-words">{c.path}</span>
                </label>
              </li>
            ))}
          </ul>
        </>
      )}
    </FormSheet>
  )
}
