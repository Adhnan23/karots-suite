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

/** Each slip's print endpoint. POST prints it; POST …?preview=1 answers with the
 * slip as the paper shows it (SlipPaper). */
export const slipPath = {
  sale: (id: number) => `/till/sales/${id}/print`,
  money: (id: number) => `/till/money-receipts/${id}/print`,
  refund: (saleId: number) => `/till/sales/${saleId}/refund-print`,
  debt: (paymentId: number) => `/till/customer-payments/${paymentId}/print`,
  warranty: (claimId: number) => `/till/warranty-claims/${claimId}/print`,
}

/** Fire-and-forget: the sale is saved; only the paper is at stake. kick folds the
 * drawer pulse into this ONE printer job (fresh cash sale in auto-print mode). */
export const printSlip = (path: string, kick = false, size?: string) => send(`${path}${q(kick, size)}`)
export const printSale = (id: number, kick = false, size?: string) => printSlip(slipPath.sale(id), kick, size)
export const printMoneySlip = (id: number, kick = false, size?: string) => printSlip(slipPath.money(id), kick, size)
export const printRefund = (saleId: number, kick = false, size?: string) => printSlip(slipPath.refund(saleId), kick, size)
export const printDebtSlip = (paymentId: number, kick = false, size?: string) => printSlip(slipPath.debt(paymentId), kick, size)
export const printWarrantySlip = (claimId: number, size?: string) => printSlip(slipPath.warranty(claimId), false, size)
