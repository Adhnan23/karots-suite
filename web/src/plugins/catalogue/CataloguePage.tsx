import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { PageHead } from '@/components/admin/PageHead'
import { type Status, statusQuery } from './api'

const when = (iso: string | null) => (iso ? new Date(iso).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }) : 'never')

/** Apps → Catalogue: the shop's copy (version, products, last check). */
export function CataloguePage() {
  const st = useQuery(statusQuery).data
  const qc = useQueryClient()
  const [busy, setBusy] = useState(false)
  async function check() {
    setBusy(true)
    try {
      const s = await api<Status>('/x/catalogue/check', { method: 'POST' })
      qc.setQueryData(statusQuery.queryKey, s)
      if (s.error) toast.error(`Check failed: ${s.error}`)
      else toast.success(`Up to date: version ${s.version}`)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <>
      <PageHead title="Catalogue" sub="The Karots Suite catalogue, kept on this PC and checked for changes every night. Pick from it on the product form, intake, the till's quick item and a delivery." />
      {st && (
        <div className="grid max-w-xl gap-3 rounded-[var(--radius)] border border-border bg-card p-4 text-sm">
          {!st.hasAddress && <p className="font-semibold text-warning">This build has no catalogue address: ask Karots for an update.</p>}
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
            <dt className="text-muted-foreground">Version</dt><dd className="font-semibold">{st.version || 'not downloaded yet'}</dd>
            <dt className="text-muted-foreground">Products</dt><dd className="font-semibold">{st.count.toLocaleString()}</dd>
            <dt className="text-muted-foreground">Last check</dt><dd>{when(st.checkedAt)}</dd>
            <dt className="text-muted-foreground">Last change</dt><dd>{when(st.syncedAt)}</dd>
          </dl>
          {st.error && <p className="text-destructive">Last check failed: {st.error}</p>}
          {st.hasAddress && <Button className="justify-self-start" disabled={busy} onClick={() => void check()}>{busy ? 'Checking…' : 'Check now'}</Button>}
        </div>
      )}
    </>
  )
}
