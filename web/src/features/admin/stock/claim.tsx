import { useState } from 'react'
import { toast } from 'sonner'
import type { LossRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { RecoverSheet } from '@/features/reports/ops/RecoverSheet'
import { dec, qtyText } from '@/lib/money'

const ASK = 'Will the supplier replace it or pay for it? You can also record that later in Reports → Losses & Recovery.'

/** After a write-off: ask whether the supplier will replace it or pay for it
 * (in full or part). "Record it" opens the Recover sheet for that loss; it also
 * waits in Reports → Losses & Recovery for later. inline: ask in `banner`
 * instead of a toast (inside a dialog, where a toast can't be clicked). */
export function useSupplierClaim(sym: string, inline = false) {
  const [pending, setPending] = useState<LossRow | null>(null)
  const [loss, setLoss] = useState<LossRow | null>(null)
  const ask = (lossId: number, product: string, qty: string, value: number) => {
    const row: LossRow = { kind: 'damage', sourceId: lossId, id: lossId, product, qty, loss: dec(value), note: '', by: '', at: '', outcome: '', recovered: '' }
    if (inline) return setPending(row)
    toast.success(`${product}: ${qtyText(qty)} written off`, { duration: 12_000, description: ASK, action: { label: 'Record it', onClick: () => setLoss(row) } })
  }
  const sheet = loss && <RecoverSheet loss={loss} sym={sym} onClose={() => setLoss(null)} />
  const banner = pending && (
    <div role="status" className="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-success p-2.5 text-sm">
      <span><b>{qtyText(pending.qty)} written off.</b> {ASK}</span>
      <div className="flex justify-end gap-1.5">
        <Button type="button" variant="ghost" size="sm" onClick={() => setPending(null)}>Later</Button>
        <Button type="button" size="sm" onClick={() => { setLoss(pending); setPending(null) }}>Record it</Button>
      </div>
    </div>
  )
  return { ask, sheet, banner }
}
