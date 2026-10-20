import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Cloud, Copy, FolderOpen, History, RotateCcw, Usb } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { BackupCopyRow, FetchedBackup } from '@/api/types.gen'
import { meQuery } from '@/api/session'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState } from '@/components/admin/PageHead'
import { PickerShell } from '@/components/admin/PickerShell'
import { Button } from '@/components/ui/button'
import { stamp } from '@/features/money/format'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { copiesQuery, copyBackupsQuery, copyStatus } from './copies'
import { fileSize } from './queries'

/** Setup → Data: where the automatic backups are also copied — the owner's
 * copy folder (USB stick / network folder) and, when Karots sold it, Karots
 * cloud. Each place can give a backup back for the normal restore. */
export function CopiesCard({ onRestore }: { onRestore: (name: string) => void }) {
  const rows = useQuery(copiesQuery).data
  const folder = rows?.find((r) => r.kind === 'folder' && r.path)
  const cloud = rows?.find((r) => r.kind === 'cloud')
  const [edit, setEdit] = useState(false)
  const refresh = useRefresh()
  if (!rows) return <p className="text-sm text-muted-foreground">…</p>
  return (
    <div className="grid gap-3 md:grid-cols-2">
      <Place icon={<Usb size={17} />} title="Copy folder"
        body={folder
          ? <><b className="block break-all font-mono text-sm">{folder.path}</b><Status row={folder} /></>
          : <p className="text-sm text-muted-foreground">Not set — add a USB stick or network folder, so a broken disk doesn&apos;t take the backups too.</p>}>
        {folder ? (
          <>
            <Switch label="Copy every backup here" row={folder} onChange={(on) => api<BackupCopyRow>('/admin/data/copies/folder', { method: 'PUT', json: { path: folder.path, enabled: on } })} />
            <Actions row={folder} onRestore={onRestore}>
              <Button variant="ghost" className="h-9 gap-1.5 px-3" onClick={() => setEdit(true)}><FolderOpen size={15} /> Change folder</Button>
              <ConfirmButton plain className="h-9 px-3" label="Clear" title="Stop copying to this folder?"
                body="The copies already there stay. Backups on this computer aren't touched." confirm="Clear"
                onConfirm={async () => { await api('/admin/data/copies/folder', { method: 'DELETE' }); await refresh() }} />
            </Actions>
          </>
        ) : (
          <Button variant="outline" className="h-10 gap-1.5 justify-self-start px-4 font-semibold" onClick={() => setEdit(true)}><FolderOpen size={16} /> Set a folder</Button>
        )}
      </Place>
      {cloud && (
        <Place icon={<Cloud size={17} />} title="Karots cloud" body={<><p className="text-sm text-muted-foreground">A locked copy, about once a day, kept 30 days by Karots.</p><Status row={cloud} /></>}>
          <CloudSwitch row={cloud} />
          <Actions row={cloud} onRestore={onRestore} />
        </Place>
      )}
      {edit && <FolderSheet path={folder?.path ?? ''} onClose={() => setEdit(false)} />}
    </div>
  )
}

function useRefresh() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ queryKey: ['admin', 'data'] })
}

function Place({ icon, title, body, children }: { icon: ReactNode; title: string; body: ReactNode; children: ReactNode }) {
  return (
    <section aria-label={title} className="grid content-start gap-2.5 rounded-[calc(var(--radius)-4px)] border border-border bg-background/40 p-3">
      <h3 className="flex items-center gap-2 text-sm font-bold"><span className="text-brand">{icon}</span>{title}</h3>
      <div className="min-w-0">{body}</div>
      {children}
    </section>
  )
}

function Status({ row }: { row: BackupCopyRow }) {
  const s = copyStatus(row)
  return <p className={cn('mt-0.5 text-[13px] font-semibold', s.tone === 'warn' ? 'text-warning' : s.tone === 'ok' ? 'text-success' : 'text-faint')}>{s.text}</p>
}

function Switch({ label, row, onChange, disabled, hint }: { label: string; row: BackupCopyRow; onChange: (on: boolean) => Promise<unknown>; disabled?: boolean; hint?: string }) {
  const refresh = useRefresh()
  return (
    <Toggle label={label} hint={hint} checked={row.enabled} disabled={disabled}
      onChange={(on) => void onChange(on).then(refresh, (e) => toast.error(errMsg(e)))} />
  )
}

/** Only Karots switches the cloud on (a paid service); any admin may switch it off. */
function CloudSwitch({ row }: { row: BackupCopyRow }) {
  const system = useQuery(meQuery).data?.flags.isSystem ?? false
  return (
    <Switch label="Copy to Karots cloud" row={row} disabled={!row.enabled && !system} hint={!row.enabled && !system ? 'Ask Karots to switch it on' : undefined}
      onChange={(on) => api<BackupCopyRow>('/admin/data/copies/cloud', { method: 'PUT', json: { enabled: on } })} />
  )
}

function Actions({ row, onRestore, children }: { row: BackupCopyRow; onRestore: (name: string) => void; children?: ReactNode }) {
  const refresh = useRefresh()
  const [busy, setBusy] = useState(false)
  const [list, setList] = useState(false)
  const copyNow = async () => {
    setBusy(true)
    try {
      const r = await api<BackupCopyRow>(`/admin/data/copies/${row.kind}/run`, { method: 'POST' })
      if (r.lastError) toast.error(copyStatus(r).text)
      else toast.success('Copied')
      await refresh()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="flex flex-wrap gap-1.5">
      <Button variant="outline" className="h-9 gap-1.5 px-3" disabled={busy || !row.enabled} onClick={() => void copyNow()}>
        <Copy size={15} /> {busy ? 'Copying…' : 'Copy now'}
      </Button>
      <Button variant="outline" className="h-9 gap-1.5 px-3" onClick={() => setList(true)}><History size={15} /> Backups here…</Button>
      {children}
      {list && <CopyBackups row={row} onClose={() => setList(false)} onRestore={(n) => { setList(false); onRestore(n) }} />}
    </div>
  )
}

/** The backups a copy place holds; Restore fetches one into this computer's
 * backups folder, then the normal restore takes over. */
function CopyBackups({ row, onClose, onRestore }: { row: BackupCopyRow; onClose: () => void; onRestore: (name: string) => void }) {
  const q = useQuery(copyBackupsQuery(row.kind))
  const refresh = useRefresh()
  const [busy, setBusy] = useState('')
  const fetchOne = async (name: string) => {
    setBusy(name)
    try {
      const r = await api<FetchedBackup>(`/admin/data/copies/${row.kind}/backups/${encodeURIComponent(name)}/fetch`, { method: 'POST' })
      await refresh()
      onRestore(r.name)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy('')
    }
  }
  return (
    <PickerShell open onOpenChange={(o) => !o && onClose()} title={row.kind === 'cloud' ? 'Backups in Karots cloud' : 'Backups in the copy folder'}>
      <div className="-mx-1 min-h-0 flex-1 overflow-auto px-1">
        {q.isError ? <p role="alert" className="text-sm font-semibold text-warning">{errMsg(q.error)}</p>
          : !q.data ? <p className="text-sm text-muted-foreground">Looking…</p>
          : q.data.length === 0 ? <EmptyState title="No backups here yet">Copy now puts the newest one here.</EmptyState>
          : (
            <ul aria-label="Copied backups" className="divide-y divide-border">
              {q.data.map((b) => (
                <li key={b.name} className="flex items-center gap-3 py-2.5">
                  <div className="min-w-0 flex-1">
                    <b className="block truncate text-sm">{stamp(b.modifiedAt)}</b>
                    <span className="block truncate text-xs text-faint">{b.name} · {fileSize(b.size)}</span>
                  </div>
                  <Button variant="outline" className="h-9 gap-1.5 px-3" disabled={!!busy} onClick={() => void fetchOne(b.name)}>
                    <RotateCcw size={15} /> {busy === b.name ? 'Fetching…' : 'Restore'}
                  </Button>
                </li>
              ))}
            </ul>
          )}
      </div>
    </PickerShell>
  )
}

function FolderSheet({ path, onClose }: { path: string; onClose: () => void }) {
  const refresh = useRefresh()
  const [value, setValue] = useState(path)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const save = async () => {
    setBusy(true)
    setError('')
    try {
      await api<BackupCopyRow>('/admin/data/copies/folder', { method: 'PUT', json: { path: value, enabled: true } })
      await refresh()
      toast.success('Copy folder saved')
      onClose()
    } catch (e) {
      setError(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Copy folder" submit={busy ? 'Testing…' : 'Save'} busy={busy} onSubmit={() => void save()}>
      <Field label="Folder" hint="The full path on this computer — a USB stick or a network folder. A new folder is made if its parent is there.">
        <input className={cn(INPUT, 'font-mono')} value={value} autoFocus spellCheck={false} name="ks-copy-folder"
          placeholder="E:\Karots copies" aria-invalid={!!error} onChange={(e) => setValue(e.target.value)} />
      </Field>
      {error && <p role="alert" className="text-sm font-semibold text-danger">{error}</p>}
    </FormSheet>
  )
}
