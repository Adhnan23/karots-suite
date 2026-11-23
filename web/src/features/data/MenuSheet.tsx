import { useQueryClient } from '@tanstack/react-query'
import { Upload } from 'lucide-react'
import { useRef, useState } from 'react'
import { toast } from 'sonner'
import { apiUpload } from '@/api/client'
import type { MenuImportResult } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ExportMenu } from '@/features/reports/kit/ExportMenu'
import { errMsg } from '@/features/till/api'

const n = (k: number, one: string, many = one + 's') => `${k} ${k === 1 ? one : many}`

/** Data → Spreadsheets → Cashier menu (docs/rules/menu.md): the till's whole menu
 *  as a sheet. An import replaces the whole menu, so it is previewed first. */
export function MenuSheet() {
  const input = useRef<HTMLInputElement>(null)
  const qc = useQueryClient()
  const [pending, setPending] = useState<{ file: File; preview: MenuImportResult } | null>(null)
  const [busy, setBusy] = useState(false)

  const preview = async (picked: File) => {
    try {
      // The bytes as previewed: a file edited on disk after the preview must not be what Replace sends (MENU-3).
      const file = new File([await picked.arrayBuffer()], picked.name, { type: picked.type })
      setPending({ file, preview: await apiUpload<MenuImportResult>('/admin/data/menu/import?dry=1', 'file', file) })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  const replace = async () => {
    if (!pending) return
    setBusy(true)
    try {
      const r = await apiUpload<MenuImportResult>('/admin/data/menu/import', 'file', pending.file)
      toast.success(`Menu replaced: ${n(r.groups, 'group')}, ${n(r.items, 'item')}`)
      const unseen = r.skipped.filter((x) => !pending.preview.skipped.includes(x))
      if (unseen.length > 0) toast.warning(`${n(unseen.length, 'line')} skipped that the preview didn't show: ${unseen.join('; ')}`)
      setPending(null)
      void qc.invalidateQueries()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  const p = pending?.preview
  return (
    <div className="rounded-[calc(var(--radius)-4px)] border border-border p-3">
      <b className="block text-sm">Cashier menu</b>
      <p className="mb-2 text-xs text-muted-foreground">Build or rework the till's menu in a spreadsheet. Importing replaces the whole menu — you'll see what changes first.</p>
      <div className="flex flex-wrap gap-2">
        <ExportMenu url="/api/v1/admin/data/menu/export" />
        <Button variant="outline" className="h-10 gap-1.5 px-4" onClick={() => input.current?.click()}><Upload size={16} /> Import</Button>
        <input ref={input} data-sheet="menu" type="file" hidden accept=".csv,.xlsx,.ods" onChange={(e) => { const f = e.target.files?.[0]; e.target.value = ''; if (f) void preview(f) }} />
      </div>
      <Dialog open={!!p} onOpenChange={(o) => !o && !busy && setPending(null)}>
        <DialogContent className="sm:max-w-[460px]">
          <DialogHeader>
            <DialogTitle>Replace the cashier menu?</DialogTitle>
            <DialogDescription>The till's menu becomes exactly what this sheet says. Products, stock and sales don't change.</DialogDescription>
          </DialogHeader>
          {p && (
            <div className="grid min-w-0 gap-2 text-sm">
              <p><b>{n(p.groups, 'group')}</b> and <b>{n(p.items, 'item')}</b> in the new menu.</p>
              {(p.removedGroups > 0 || p.removedItems > 0) && (
                <p className="text-muted-foreground">{n(p.removedGroups, 'group')} removed, {n(p.removedItems, 'item')} removed from today's menu.</p>
              )}
              {p.skipped.length > 0 && (
                <div className="min-w-0">
                  <p className="font-medium">{n(p.skipped.length, 'line')} skipped</p>
                  <ul className="mt-1 max-h-40 overflow-y-auto break-words rounded-[calc(var(--radius)-4px)] border border-border bg-background/40 p-2 text-xs text-muted-foreground">
                    {p.skipped.map((s) => <li key={s}>{s}</li>)}
                  </ul>
                </div>
              )}
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" disabled={busy} onClick={() => setPending(null)}>Cancel</Button>
            <Button disabled={busy} onClick={() => void replace()}>Replace the menu</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
