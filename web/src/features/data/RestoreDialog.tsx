import { LogIn, RotateCcw } from 'lucide-react'
import { useState } from 'react'
import { api, apiUpload } from '@/api/client'
import type { RestoreResult } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { errMsg } from '@/features/till/api'
import { DataCheckList } from './DataCheck'

/** What to restore: an uploaded file or one already in the backup folder. */
export type RestoreSource = { file: File } | { name: string }

const WORD = 'RESTORE'

/** Restore behind a typed confirmation. It replaces every record in the shop,
 * so the server first saves a copy of what's there (the undo) and then ends
 * the session — the dialog ends on the report and "Sign in again". */
export function RestoreDialog({ source, onClose }: { source: RestoreSource; onClose: () => void }) {
  const [typed, setTyped] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState<RestoreResult | null>(null)
  const label = 'file' in source ? source.file.name : source.name

  const run = async () => {
    setBusy(true)
    setError(null)
    try {
      setDone('file' in source
        ? await apiUpload<RestoreResult>('/admin/data/restore', 'backup', source.file)
        : await api<RestoreResult>(`/admin/data/backups/${encodeURIComponent(source.name)}/restore`, { method: 'POST' }))
    } catch (e) {
      setError(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  // The session is gone and the till's saved carts belong to the old data.
  const signIn = () => {
    try { sessionStorage.clear() } catch { /* private mode */ }
    window.location.assign('/login')
  }

  return (
    <Dialog open onOpenChange={(o) => !o && !busy && !done && onClose()}>
      <DialogContent className="max-h-[90dvh] gap-3 overflow-y-auto p-5 sm:max-w-lg" onInteractOutside={(e) => done && e.preventDefault()}>
        {done ? (
          <>
            <DialogTitle className="font-display text-lg font-bold">Restored</DialogTitle>
            <DialogDescription>
              {done.report.rows.toLocaleString()} records in {done.report.tables} tables came back from <b className="break-all">{label}</b>.
              What was here before is saved as <b className="break-all">{done.safetyCopy}</b> in the backup folder.
            </DialogDescription>
            {done.report.skippedTables.length > 0 && (
              <Note title="Not in this version, left out">{done.report.skippedTables.join(', ')}</Note>
            )}
            {done.report.droppedColumns.length > 0 && (
              <Note title="Fields this version doesn't keep">{done.report.droppedColumns.join(', ')}</Note>
            )}
            <section>
              <h3 className="mb-1.5 text-sm font-bold">Data check</h3>
              <DataCheckList findings={done.check} />
            </section>
            <Button className="mt-1 h-11 gap-2 font-bold" onClick={signIn}><LogIn size={17} /> Sign in again</Button>
          </>
        ) : (
          <>
            <DialogTitle className="font-display text-lg font-bold">Restore {label}?</DialogTitle>
            <DialogDescription>
              Every sale, product, customer, supplier, user and setting is replaced by what's in this backup.
              A copy of what's here now is saved first, so you can undo by restoring that copy. You sign in again, and so does anyone whose account the backup changes.
            </DialogDescription>
            <label className="grid gap-1.5 text-sm font-semibold">
              Type {WORD} to go ahead
              <input className={INPUT} value={typed} autoComplete="off" spellCheck={false} name="ks-restore-word"
                onChange={(e) => setTyped(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && typed === WORD && !busy && void run()} />
            </label>
            {error && <p role="alert" className="text-sm font-semibold text-danger">{error}</p>}
            <div className="mt-1 flex justify-end gap-2">
              <Button variant="ghost" className="h-10 px-4" disabled={busy} onClick={onClose}>Keep my data</Button>
              <Button className="h-10 gap-1.5 bg-danger px-4 font-bold text-white hover:bg-danger/90" disabled={typed !== WORD || busy} onClick={() => void run()}>
                <RotateCcw size={16} /> {busy ? 'Restoring…' : 'Restore'}
              </Button>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

function Note({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-[calc(var(--radius)-4px)] border border-border bg-muted/40 p-2.5 text-sm">
      <b className="block text-xs font-bold uppercase tracking-wide text-muted-foreground">{title}</b>
      <span className="break-words">{children}</span>
    </div>
  )
}
