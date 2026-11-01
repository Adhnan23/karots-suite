import { useQueryClient } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { RecallRequest } from '@/api/types.gen'
import { PickerShell } from '@/components/admin/PickerShell'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'

type RecallLot = { lotId: number; product: string; supplierId: number }

/** The supplier recalled a lot: keep selling it, or stop (the till then warns,
 * as for an expired lot). It stays in stock until given back. */
export function useRecall(base: '/admin' | '/till' = '/admin') {
  const qc = useQueryClient()
  const [lot, setLot] = useState<RecallLot | null>(null)
  const [busy, setBusy] = useState(false)
  const path = (l: RecallLot) => `${base}/suppliers/${l.supplierId}/claims/lots/${l.lotId}/recall`
  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['admin'] })
    void qc.invalidateQueries({ queryKey: ['till'] })
  }
  async function recall(stopSelling: boolean) {
    if (!lot) return
    setBusy(true)
    try {
      const body: RecallRequest = { stopSelling }
      await api(path(lot), { json: body })
      setLot(null)
      refresh()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  async function undo(l: RecallLot) {
    try {
      await api(path(l), { method: 'DELETE' })
      refresh()
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  const sheet: ReactNode = lot && (
    <PickerShell fit open onOpenChange={(o) => !o && setLot(null)} title={`Recall ${lot.product}`}>
      <div className="flex flex-col gap-3 text-sm">
        <p className="text-muted-foreground">The supplier wants this batch back. It stays in stock and waits for their rep. Is it safe to keep selling until then?</p>
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" disabled={busy} onClick={() => void recall(false)}>Keep selling</Button>
          <Button type="button" disabled={busy} onClick={() => void recall(true)}>Stop selling</Button>
        </div>
      </div>
    </PickerShell>
  )
  return { ask: setLot as (l: RecallLot) => void, undo, sheet }
}
