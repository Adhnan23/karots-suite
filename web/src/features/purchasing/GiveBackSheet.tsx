import { useQueryClient } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ConsumeResult, GiveBackRequest, LossRow } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { PickerShell } from '@/components/admin/PickerShell'
import { ClaimPrompt } from '@/features/admin/stock/claim'
import { RecoverSheet } from '@/features/reports/ops/RecoverSheet'
import { errMsg } from '@/features/till/api'
import { dec, num, qtyText } from '@/lib/money'

export type GiveLot = { lotId: number; productId: number; product: string; unit: string; qty: string; expiry: string; cost: string; supplierId: number; supplier: string }

/** What the count will do, said before saving. */
export function countLine(counted: string, system: string) {
  if (counted === '') return ''
  const c = num(counted)
  const diff = c - num(system)
  if (c === 0) return `Nothing goes back. The ${qtyText(system)} in the system move to this product's other batches, or come off the stock if it has none.`
  if (diff > 0) return `${qtyText(String(diff))} more than the system: taken from this product's other batches, or added to the stock if they can't cover it.`
  if (diff < 0) return `${qtyText(String(-diff))} fewer than the system: moved to this product's other batches, or taken off the stock if it has none.`
  return 'Matches the system.'
}

/** Count a lot on the shelf; everything counted goes back to its supplier. */
function GiveBackSheet({ lot, base, onClose, onDone }: { lot: GiveLot; base: '/admin' | '/till'; onClose: () => void; onDone: (lossId: number, counted: string) => void }) {
  const [counted, setCounted] = useState('')
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      const body: GiveBackRequest = { counted }
      const r = await api<ConsumeResult>(`${base}/suppliers/${lot.supplierId}/claims/lots/${lot.lotId}/give-back`, { json: body })
      onDone(r.lossId, counted)
    } catch (e) {
      toast.error(errMsg(e))
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Give back: ${lot.product}`} submit="Give back" busy={busy || counted === ''} onSubmit={save}>
      <p className="text-sm text-muted-foreground">To {lot.supplier}{lot.expiry && ` · expires ${lot.expiry}`} · {qtyText(lot.qty)} {lot.unit} in the system</p>
      <Field label="Counted on the shelf" hint={countLine(counted, lot.qty) || 'Batches get mixed, so count what is really there. All of it goes back.'}>
        <input autoFocus aria-label="Counted on the shelf" inputMode="decimal" value={counted} onChange={(e) => setCounted(e.target.value.replace(/[^\d.]/g, ''))} className={INPUT} />
      </Field>
    </FormSheet>
  )
}

/** Give a lot back: the count, then "settle now or later?" as a prompt, then
 * the Recover form. `give` starts it; `sheet` renders whichever step is open. */
export function useGiveBack(sym: string, base: '/admin' | '/till' = '/admin') {
  const qc = useQueryClient()
  const [lot, setLot] = useState<GiveLot | null>(null)
  const [asked, setAsked] = useState<{ row: LossRow; supplierId: number } | null>(null)
  const [settling, setSettling] = useState<{ row: LossRow; supplierId: number } | null>(null)
  function done(g: GiveLot, lossId: number, counted: string) {
    setLot(null)
    void qc.invalidateQueries({ queryKey: ['admin'] })
    void qc.invalidateQueries({ queryKey: ['till'] })
    if (lossId === 0) {
      toast.success(`Nothing went back. ${g.product} is updated.`)
      return
    }
    setAsked({ supplierId: g.supplierId, row: { kind: 'damage', sourceId: lossId, id: lossId, productId: g.productId, product: g.product, qty: counted,
      loss: dec(num(counted) * num(g.cost)), note: '', by: '', at: '', outcome: '', recovered: '', givenBack: true } })
  }
  const sheet: ReactNode = (
    <>
      {lot && <GiveBackSheet lot={lot} base={base} onClose={() => setLot(null)} onDone={(id, counted) => done(lot, id, counted)} />}
      {asked && (
        <PickerShell fit open onOpenChange={(o) => !o && setAsked(null)} title="Given back">
          <ClaimPrompt title={`${asked.row.product}: ${qtyText(asked.row.qty)} given back.`} goLabel="Settle now" later={() => setAsked(null)}
            body="It waits on this supplier until settled. Has the supplier replaced it or paid for it yet?" go={() => { setSettling(asked); setAsked(null) }} />
        </PickerShell>
      )}
      {settling && <RecoverSheet loss={settling.row} sym={sym} supplierId={settling.supplierId} base={base} onClose={() => { setSettling(null); void qc.invalidateQueries({ queryKey: [base.slice(1)] }) }} />}
    </>
  )
  return { give: setLot as (l: GiveLot) => void, sheet }
}
