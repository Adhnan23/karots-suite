import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { FreezeResultDTO, FrozenListDTO } from '@/api/types.gen'
import { meQuery } from '@/api/session'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ToShelfDialog } from '@/features/freeze/ToShelfDialog'
import { UnfreezeDialog } from '@/features/freeze/UnfreezeDialog'
import { refreshStock } from '@/features/freeze/api'
import { errMsg } from '@/features/till/api'
import { qtyText } from '@/lib/money'

export const adminFrozenQuery = { queryKey: ['admin', 'frozen'], queryFn: () => api<FrozenListDTO>('/admin/stock/frozen') }

/** Say what a freeze did, and who a plugin kept counted. */
export function freezeToast(r: FreezeResultDTO) {
  const refused = r.refused.length ? ` · ${r.refused.length} kept counted: ${r.refused.slice(0, 3).map((x) => `${x.name} (${x.reason})`).join(', ')}${r.refused.length > 3 ? '…' : ''}` : ''
  toast.success(`${r.frozen} frozen${refused}`)
}

/** Stock freeze: how far onboarding has got, and the shop-wide switch (admin). */
export function FrozenView() {
  const qc = useQueryClient()
  const data = useQuery(adminFrozenQuery).data
  const admin = useQuery(meQuery).data?.user.role === 'admin'
  const [act, setAct] = useState<{ id: number; kind: 'unfreeze' | 'shelf' } | null>(null)
  const [warn, setWarn] = useState(false)
  const [busy, setBusy] = useState(false)
  async function shop(on: boolean) {
    setBusy(true)
    try {
      if (on) freezeToast(await api<FreezeResultDTO>('/admin/stock/freeze-shop', { method: 'POST' }))
      else toast.success(`${(await api<{ unfrozen: number }>('/admin/stock/unfreeze-shop', { method: 'POST' })).unfrozen} unfrozen: every sale counts now`)
      refreshStock(qc)
      setWarn(false)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  const rows = data?.products ?? []
  const cur = rows.find((r) => r.id === act?.id)
  return (
    <>
      <div className="mb-3 flex flex-col gap-2 rounded-[var(--radius)] border border-border bg-card p-4 md:flex-row md:items-center">
        <p className="flex-1 text-sm text-muted-foreground">
          {data?.shopFrozen
            ? 'The shop is frozen: sales don’t touch stock, and new products start frozen. Unfreeze each product when its shelf runs low.'
            : 'Count only what doesn’t move (the back), then freeze: the till sells freely while the front shelf isn’t counted.'}
        </p>
        {admin && (data?.shopFrozen
          ? <Button variant="outline" disabled={busy} onClick={() => (rows.length ? setWarn(true) : void shop(false))}>Unfreeze the shop…</Button>
          : <Button disabled={busy} onClick={() => void shop(true)}>Freeze the shop</Button>)}
      </div>
      <DataList label="Frozen products" rows={rows} rowKey={(r) => r.id}
        empty={<EmptyState title={data ? 'Nothing is frozen' : 'Loading…'}>{data && 'Every sale takes stock out as usual.'}</EmptyState>}
        columns={[
          { key: 'name', header: 'Product', cell: (r) => <b>{r.name}</b> },
          { key: 'stock', header: 'Counted', num: true, cell: (r) => <span className="tabular-nums">{qtyText(r.stock)} <small className="text-faint">{r.unit}</small></span> },
          { key: 'since', header: 'Frozen since', cell: (r) => <span className="text-muted-foreground">{r.since ? new Date(r.since).toLocaleDateString([], { day: 'numeric', month: 'short' }) : '—'}</span> },
          { key: 'act', header: '', wide: true, interactive: true, className: 'w-[1%]', cell: (r) => (
            <span className="flex gap-1">
              <Button variant="ghost" className="h-9 px-3" onClick={() => setAct({ id: r.id, kind: 'shelf' })}>To the shelf</Button>
              <Button variant="ghost" className="h-9 px-3" onClick={() => setAct({ id: r.id, kind: 'unfreeze' })}>Unfreeze</Button>
            </span>
          ) },
        ]} />
      {cur && act?.kind === 'unfreeze' && <UnfreezeDialog id={cur.id} name={cur.name} unit={cur.unit} onClose={() => setAct(null)} />}
      {cur && act?.kind === 'shelf' && <ToShelfDialog id={cur.id} onClose={() => setAct(null)} />}
      {warn && (
        <Dialog open onOpenChange={(o) => !o && setWarn(false)}>
          <DialogContent className="sm:max-w-[440px]">
            <DialogHeader>
              <DialogTitle>Unfreeze the whole shop?</DialogTitle>
              <DialogDescription>
                {rows.length} {rows.length === 1 ? 'product is' : 'products are'} still frozen. Their stock stays at what was counted in the back:
                count their front shelves first, or unfreeze them one by one with the shelf count.
              </DialogDescription>
            </DialogHeader>
            <ul aria-label="Still frozen" className="max-h-48 overflow-auto rounded-[var(--radius-sm)] bg-muted px-3 py-2 text-sm">
              {rows.map((r) => <li key={r.id} className="flex justify-between gap-3 py-0.5"><span className="truncate">{r.name}</span><span className="tabular-nums text-muted-foreground">{qtyText(r.stock)} {r.unit}</span></li>)}
            </ul>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setWarn(false)}>Keep frozen</Button>
              <Button variant="destructive" disabled={busy} onClick={() => void shop(false)}>Unfreeze all</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </>
  )
}
