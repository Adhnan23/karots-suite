import { useQuery } from '@tanstack/react-query'
import { ArrowUp, Folder, FolderPlus } from 'lucide-react'
import { useState } from 'react'
import { api } from '@/api/client'
import type { FolderListing } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { PickerShell } from '@/components/admin/PickerShell'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'

/** Choose a folder on the shop's main PC (where the server runs — the browser
 * can't give a real path, and may be a phone). Opens at `start`, else the USB
 * drives, else Home, else the first drive. */
export function FolderPicker({ start, onPick, onClose }: { start: string; onPick: (path: string) => void; onClose: () => void }) {
  const [path, setPath] = useState<string | null>(start || null)
  const q = useQuery({
    queryKey: ['admin', 'data', 'folders', path ?? ''],
    queryFn: () => api<FolderListing>(`/admin/data/folders?path=${encodeURIComponent(path ?? '')}`),
  })
  const l = q.data
  // First open with no folder typed: the USB drives when the PC has them, else Home.
  const first = l?.roots.find((r) => r.name === 'USB drives') ?? l?.roots.find((r) => r.name === 'Home')
  if (path === null && first) setPath(first.path)
  const [name, setName] = useState('')
  const [err, setErr] = useState('')
  const join = (n: string) => (l ? l.path.replace(/[\\/]$/, '') + (l.path.includes('\\') ? '\\' : '/') + n : n)
  const make = async () => {
    setErr('')
    try {
      const r = await api<FolderListing>('/admin/data/folders', { method: 'POST', json: { path: l?.path ?? '', name } })
      setName('')
      setPath(r.path)
    } catch (e) {
      setErr(errMsg(e))
    }
  }
  return (
    <PickerShell open onOpenChange={(o) => !o && onClose()} title="Choose the copy folder">
      <div className="flex flex-wrap gap-1.5">
        {l?.roots.map((r) => (
          <Button key={r.path} variant="outline" className={cn('h-8 px-3 text-[13px]', l.path === r.path && 'border-selected')} onClick={() => setPath(r.path)}>{r.name}</Button>
        ))}
      </div>
      <b className="break-all font-mono text-[13px]">{l?.path ?? '…'}</b>
      <div className="-mx-1 min-h-0 flex-1 overflow-auto px-1">
        {q.isError && <p role="alert" className="text-sm font-semibold text-warning">{errMsg(q.error)}</p>}
        <ul aria-label="Folders" className="divide-y divide-border">
          {l?.parent && (
            <li><button type="button" className="flex w-full items-center gap-2 py-2.5 text-left text-sm font-semibold" onClick={() => setPath(l.parent)}><ArrowUp size={16} className="text-faint" /> Up</button></li>
          )}
          {l?.folders.map((f) => (
            <li key={f}><button type="button" className="flex w-full items-center gap-2 py-2.5 text-left text-sm" onClick={() => setPath(join(f))}><Folder size={16} className="shrink-0 text-brand" /><span className="truncate">{f}</span></button></li>
          ))}
          {l && l.folders.length === 0 && <li className="py-2.5 text-sm text-faint">No folders here — make one below, or use this one.</li>}
        </ul>
      </div>
      <div className="flex gap-1.5">
        <input className={INPUT} value={name} placeholder="New folder, e.g. Karots copies" name="ks-new-folder" autoComplete="off" spellCheck={false}
          onChange={(e) => setName(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && name.trim() && void make()} />
        <Button variant="outline" className="h-[var(--control-h)] shrink-0 gap-1.5 px-3" disabled={!name.trim()} onClick={() => void make()}><FolderPlus size={16} /> Make</Button>
      </div>
      {err && <p role="alert" className="text-sm font-semibold text-danger">{err}</p>}
      <Button className="h-11 font-bold" disabled={!l} onClick={() => l && onPick(l.path)}>Use this folder</Button>
    </PickerShell>
  )
}
