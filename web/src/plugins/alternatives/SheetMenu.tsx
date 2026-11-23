import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, FileSpreadsheet, Upload } from 'lucide-react'
import { useRef, useState } from 'react'
import { toast } from 'sonner'
import { apiUpload } from '@/api/client'
import { meQuery } from '@/api/session'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { errMsg } from '@/features/till/api'
import type { SheetImport } from './types'

const EXPORT = '/api/v1/x/alternatives/sheet'
const IMPORT = '/x/alternatives/sheet/import'
const n = (k: number, one: string, many = one + 's') => `${k} ${k === 1 ? one : many}`
const LIST = 'mt-1 max-h-40 overflow-y-auto break-words rounded-[calc(var(--radius)-4px)] border border-border bg-background/40 p-2 text-xs text-muted-foreground'

/** Sheet ▾ on Apps → Alternatives (docs/rules/plugins/alternatives.md ALT-9…18): the
 *  whole set out as a sheet; in, after a preview, it becomes the whole set. Import is
 *  admin only (ALT-16). */
export function SheetMenu() {
  const admin = useQuery(meQuery).data?.user.role === 'admin'
  const input = useRef<HTMLInputElement>(null)
  const qc = useQueryClient()
  const [pending, setPending] = useState<{ file: File; preview: SheetImport } | null>(null)
  const [busy, setBusy] = useState(false)

  const preview = async (picked: File) => {
    try {
      // The bytes as previewed: a file edited on disk afterwards must not be what Replace sends (ALT-11).
      const file = new File([await picked.arrayBuffer()], picked.name, { type: picked.type })
      setPending({ file, preview: await apiUpload<SheetImport>(`${IMPORT}?dry=1`, 'file', file) })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  const replace = async () => {
    if (!pending) return
    setBusy(true)
    try {
      const r = await apiUpload<SheetImport>(IMPORT, 'file', pending.file)
      toast.success(`Alternatives replaced: ${n(r.groups, 'group')}, ${n(r.members, 'product')}`)
      const unseen = r.skipped.filter((x) => !pending.preview.skipped.includes(x))
      if (unseen.length > 0) toast.warning(`${n(unseen.length, 'line')} skipped that the preview didn't show: ${unseen.join('; ')}`)
      setPending(null)
      void qc.invalidateQueries({ queryKey: ['x', 'alternatives'] })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  const p = pending?.preview
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="secondary" className="h-10 gap-1.5 px-4"><FileSpreadsheet size={16} /> Sheet</Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-56">
          {admin && <>
            <DropdownMenuItem onSelect={() => input.current?.click()}><Upload size={15} /> Import from a sheet…</DropdownMenuItem>
            <DropdownMenuSeparator />
          </>}
          <DropdownMenuLabel className="text-xs text-muted-foreground">Export every group</DropdownMenuLabel>
          {(['csv', 'xlsx', 'ods'] as const).map((f) => (
            <DropdownMenuItem key={f} asChild><a href={`${EXPORT}?format=${f}`} download><Download size={15} /> {f.toUpperCase()}</a></DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      {admin && <input ref={input} data-sheet="alternatives" type="file" hidden accept=".csv,.xlsx,.ods"
        onChange={(e) => { const f = e.target.files?.[0]; e.target.value = ''; if (f) void preview(f) }} />}
      <Dialog open={!!p} onOpenChange={(o) => !o && !busy && setPending(null)}>
        <DialogContent className="sm:max-w-[480px]">
          <DialogHeader>
            <DialogTitle>Replace your alternatives?</DialogTitle>
            <DialogDescription>Your groups become exactly what this sheet says. Products, stock and sales don't change.</DialogDescription>
          </DialogHeader>
          {p && (
            <div className="grid min-w-0 gap-2 text-sm">
              <p><b>{n(p.groups, 'group')}</b> and <b>{n(p.members, 'product')}</b> in the new set.</p>
              <p className="text-muted-foreground">
                {n(p.groupsAdded, 'new group')}, {n(p.groupsRemoved, 'group')} removed · {n(p.placed, 'product')} placed, {p.moved} moved, {p.takenOut} taken out
              </p>
              {p.removed.length > 0 && (
                <div className="min-w-0">
                  <p className="font-medium">Removed</p>
                  <ul className={LIST}>{p.removed.map((g, i) => <li key={i}>{g}</li>)}</ul>
                </div>
              )}
              {p.skipped.length > 0 && (
                <div className="min-w-0">
                  <p className="font-medium">{n(p.skipped.length, 'line')} skipped</p>
                  <ul className={LIST}>{p.skipped.map((s) => <li key={s}>{s}</li>)}</ul>
                </div>
              )}
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" disabled={busy} onClick={() => setPending(null)}>Cancel</Button>
            <Button disabled={busy} onClick={() => void replace()}>Replace</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
