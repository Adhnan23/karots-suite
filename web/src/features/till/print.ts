import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'

const FAIL = "Receipt didn't print — check the printer is on and connected"

async function send(path: string) {
  try {
    await api(path, { method: 'POST' })
    toast.success('Receipt sent to printer')
  } catch (e) {
    toast.error(e instanceof ApiError && e.code === 'NO_PRINTER' ? e.message : FAIL)
  }
}

const q = (kick: boolean, size?: string) => {
  const p = new URLSearchParams()
  if (kick) p.set('kick', '1')
  if (size) p.set('size', size) // "58" | "80": this one job only (reprints from Receipts)
  const s = p.toString()
  return s ? `?${s}` : ''
}

/** Fire-and-forget: the sale is saved; only the paper is at stake. kick folds the
 * drawer pulse into this ONE printer job (fresh cash sale in auto-print mode). */
export const printSale = (id: number, kick = false, size?: string) => send(`/till/sales/${id}/print${q(kick, size)}`)
export const printMoneySlip = (id: number, kick = false, size?: string) => send(`/till/money-receipts/${id}/print${q(kick, size)}`)
export const printRefund = (saleId: number, kick = false, size?: string) => send(`/till/sales/${saleId}/refund-print${q(kick, size)}`)
export const printDebtSlip = (paymentId: number, kick = false, size?: string) => send(`/till/customer-payments/${paymentId}/print${q(kick, size)}`)
export const printWarrantySlip = (claimId: number, size?: string) => send(`/till/warranty-claims/${claimId}/print${q(false, size)}`)
