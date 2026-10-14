import { toast } from 'sonner'
import { ApiError } from '@/api/client'
import type { SaleHookCtx } from '../slots'
import { recordDispense } from './api'
import type { SalePatient } from './sale'

/** After a sale with a patient on it, record what they were given: the lines added
 * from a script say which script line; the server fills the rest by medicine group.
 * Best-effort — the sale stands; a failure says how to record it later. */
export async function afterSale({ saleId, lines, meta }: SaleHookCtx) {
  const p = meta.pharmacy as SalePatient | undefined
  if (!p?.patientId) return
  const tags = lines.flatMap((l) => {
    const rx = (l.plugin?.pharmacy as { rxItemId?: number } | undefined)?.rxItemId
    return rx ? [{ productId: l.productId, rxItemId: rx }] : []
  })
  try {
    await recordDispense({ saleId, patientId: p.patientId, lines: tags })
  } catch (e) {
    if (e instanceof ApiError && e.status === 422) return // nothing medicinal on the sale
    toast.warning(`Sale saved, but ${p.name}’s record wasn’t — 💊 Patient → Record for a past receipt`)
  }
}
