import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CloudDownload, RefreshCw } from 'lucide-react'
import { useEffect, type ReactNode } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { UpdateStatusDTO } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Button } from '@/components/ui/button'
import { stamp } from '@/features/money/format'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { updateQuery } from './queries'

/** Setup → Data: this copy's version and its updates — check, read what's
 * new, Update. The shop never installs by itself. */
export function UpdateCard() {
  const qc = useQueryClient()
  const st = useQuery(updateQuery).data
  const put = (s: UpdateStatusDTO) => qc.setQueryData(updateQuery.queryKey, s)
  const check = useMutation({ mutationFn: () => api<UpdateStatusDTO>('/admin/data/update/check', { method: 'POST' }), onSuccess: put })
  const dismiss = useMutation({ mutationFn: () => api<UpdateStatusDTO>('/admin/data/update/dismiss', { method: 'POST' }), onSuccess: put })
  const enabled = st?.enabled
  const { mutate } = check
  useEffect(() => { if (enabled) mutate() }, [enabled, mutate]) // check on opening the page

  if (!st) return <p className="text-sm text-muted-foreground">…</p>
  const checked = st.checkedAt ? `Last checked ${stamp(st.checkedAt)}` : ''

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-sm font-bold">Version {st.version}</span>
        {st.kind === 'test' && <span className="rounded-full bg-warning/15 px-2 py-0.5 text-xs font-bold text-warning">Test build</span>}
      </div>

      {st.rolledBackFrom && (
        <Note tone="warn">
          The update to {st.rolledBackFrom} couldn't start, so Karots went back to {st.version}. Sales are safe. Tell support.
          <Button variant="ghost" className="ml-2 h-8 px-2" onClick={() => dismiss.mutate()}>Dismiss</Button>
        </Note>
      )}

      {!enabled ? (
        <p className="text-sm text-muted-foreground">This copy doesn't update itself — a new version is put in by whoever set it up.</p>
      ) : (
        <>
          <Status st={st} checked={checked} />
          <div className="flex flex-wrap gap-2">
            {st.state === 'available' && (
              <ConfirmButton plain className="h-10 gap-2 bg-brand px-4 font-bold text-brand-foreground hover:bg-brand/90"
                label={<><CloudDownload size={16} /> Update to {st.latest}</>}
                title={`Update to ${st.latest}?`}
                body="Tills will pause for about a minute while Karots restarts. Sales already rung up are safe — a backup is taken first."
                confirm="Update now"
                onConfirm={async () => put(await api<UpdateStatusDTO>('/admin/data/update/apply', { method: 'POST' }))} />
            )}
            {st.state !== 'installing' && (
              <Button variant="ghost" className="h-10 gap-2 px-4" disabled={check.isPending}
                onClick={() => check.mutate(undefined, { onError: (e) => toast.error(errMsg(e)) })}>
                <RefreshCw size={16} className={cn(check.isPending && 'animate-spin')} /> Check now
              </Button>
            )}
          </div>
        </>
      )}
    </div>
  )
}

function Status({ st, checked }: { st: UpdateStatusDTO; checked: string }) {
  switch (st.state) {
    case 'available':
      return (
        <div>
          <p className="font-bold">Version {st.latest} is ready</p>
          {st.notes && <pre className="mt-2 max-h-56 overflow-auto whitespace-pre-wrap rounded-[var(--radius)] bg-muted/50 p-3 font-sans text-sm">{st.notes}</pre>}
        </div>
      )
    case 'up-to-date':
      return <p className="text-sm"><b>Up to date.</b> <span className="text-muted-foreground">{checked}</span></p>
    case 'not-published':
      return <p className="text-sm text-muted-foreground">No update published for this shop yet. {checked}</p>
    case 'offline':
      return <p className="text-sm text-muted-foreground">{st.message}. {checked}</p>
    case 'bad-signature':
      return <Note tone="warn">{st.message}</Note>
    case 'installing':
      return <p className="flex items-center gap-2 text-sm font-bold"><RefreshCw size={16} className="animate-spin" /> {st.progress || 'Installing…'}</p>
    case 'failed':
      return <Note tone="warn">{st.message}</Note>
    default:
      return <p className="text-sm text-muted-foreground">Checking…</p>
  }
}

function Note({ tone, children }: { tone: 'warn'; children: ReactNode }) {
  return <div className={cn('rounded-[var(--radius)] p-3 text-sm', tone === 'warn' && 'bg-warning/15 text-warning')}>{children}</div>
}
