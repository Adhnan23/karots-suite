import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { BackupCopyRow, BackupFileRow } from '@/api/types.gen'

export const copiesQuery = queryOptions({ queryKey: ['admin', 'data', 'copies'], queryFn: () => api<BackupCopyRow[]>('/admin/data/copies') })

export const copyBackupsQuery = (kind: string) =>
  queryOptions({ queryKey: ['admin', 'data', 'copies', kind, 'backups'], queryFn: () => api<BackupFileRow[]>(`/admin/data/copies/${kind}/backups`) })

/** "just now", "10 min ago", "3 h ago", "2 days ago" */
export function ago(iso: string, now = Date.now()) {
  const min = Math.max(0, Math.round((now - new Date(iso).getTime()) / 60_000))
  if (min < 1) return 'just now'
  if (min < 60) return `${min} min ago`
  const h = Math.floor(min / 60)
  if (h < 24) return `${h} h ago`
  const d = Math.floor(h / 24)
  return `${d} day${d === 1 ? '' : 's'} ago`
}

const DAY = 24 * 60 * 60_000

/** The one line under a copy place: how its last copy went. */
export function copyStatus(r: BackupCopyRow, now = Date.now()): { text: string; tone: 'ok' | 'warn' | 'muted' } {
  if (!r.enabled) return { text: 'Switched off', tone: 'muted' }
  if (r.lastError.startsWith('not there right now')) return { text: 'Not there right now — plug the USB in', tone: 'warn' }
  if (r.lastError) return { text: `Failed: ${r.lastError}`, tone: 'warn' }
  const waiting = r.waiting === 0 ? '' : r.kind === 'cloud' ? ' · next copy due' : ` · ${r.waiting} waiting`
  if (r.lastCopiedAt) return { text: `Last copied ${ago(r.lastCopiedAt, now)}${waiting}`, tone: now - new Date(r.lastCopiedAt).getTime() > DAY ? 'warn' : 'ok' }
  return { text: `No copy yet${waiting}`, tone: 'muted' }
}
