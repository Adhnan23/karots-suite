import { useState, type ReactNode } from 'react'
import type { LossRow } from '@/api/types.gen'
import { PickerShell } from '@/components/admin/PickerShell'
import { Button } from '@/components/ui/button'
import { RecoverSheet } from '@/features/reports/ops/RecoverSheet'
import { dec, qtyText } from '@/lib/money'

const ASK = 'Will the supplier replace it or pay for it (in full or part)? You can also record that later in Reports → Losses & Recovery.'

/** The question after a write-off, as a prompt (a toast is too easy to miss). */
export function ClaimPrompt({ title, body, later, go, goLabel }: { title: string; body: string; later: () => void; go: () => void; goLabel: string }) {
  return (
    <div role="status" aria-label="Supplier claim" className="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-success p-2.5 text-sm">
      <span><b>{title}</b> {body}</span>
      <div className="flex justify-end gap-1.5">
        <Button type="button" variant="ghost" size="sm" onClick={later}>Later</Button>
        <Button type="button" size="sm" onClick={go}>{goLabel}</Button>
      </div>
    </div>
  )
}

/** After a damage write-off: ask whether the supplier will replace it or pay
 * for it. "Record it" opens the Recover sheet for that loss. inline: the
 * prompt is `banner`, placed inside an open dialog; otherwise `sheet` shows it
 * as its own small dialog. */
export function useSupplierClaim(sym: string, inline = false) {
  const [pending, setPending] = useState<LossRow | null>(null)
  const [loss, setLoss] = useState<LossRow | null>(null)
  const ask = (lossId: number, productId: number, product: string, qty: string, value: number) =>
    setPending({ kind: 'damage', sourceId: lossId, id: lossId, productId, product, qty, loss: dec(value), note: '', by: '', at: '', outcome: '', recovered: '' })
  const prompt = pending && (
    <ClaimPrompt title={`${pending.product}: ${qtyText(pending.qty)} written off.`} body={ASK} goLabel="Record it"
      later={() => setPending(null)} go={() => { setLoss(pending); setPending(null) }} />
  )
  const sheet: ReactNode = (
    <>
      {!inline && pending && <PickerShell fit open onOpenChange={(o) => !o && setPending(null)} title="Written off">{prompt}</PickerShell>}
      {loss && <RecoverSheet loss={loss} sym={sym} onClose={() => setLoss(null)} />}
    </>
  )
  return { ask, sheet, banner: inline ? prompt : null }
}
