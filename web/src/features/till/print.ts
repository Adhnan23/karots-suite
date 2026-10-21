import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { queryClient } from '@/api/query-client'
import { slipPicture } from '@/features/print/picture'
import { configQuery } from './api'

const FAIL = "Receipt didn't print — check the printer is on and connected"

async function send(path: string, blob?: Blob) {
  try {
    await api(path, { method: 'POST', blob })
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
export async function printSlip(path: string, kick = false, size?: string) {
  const cfg = await queryClient.ensureQueryData(configQuery).catch(() => null)
  // Image mode: the printer gets the same picture the preview showed. If it
  // can't be drawn, this one slip prints Classic (the kick still rides once).
  const pic = cfg?.printAs === 'image'
    ? await slipPicture(path, size).catch((e: unknown) => void console.warn('receipt picture failed — printing Classic', e))
    : undefined
  return send(`${path}${q(kick, size)}`, pic?.blob)
}
export const printSale = (id: number, kick = false, size?: string) => printSlip(slipPath.sale(id), kick, size)
export const printMoneySlip = (id: number, kick = false, size?: string) => printSlip(slipPath.money(id), kick, size)
export const printRefund = (saleId: number, kick = false, size?: string) => printSlip(slipPath.refund(saleId), kick, size)
export const printDebtSlip = (paymentId: number, kick = false, size?: string) => printSlip(slipPath.debt(paymentId), kick, size)
export const printWarrantySlip = (claimId: number, size?: string) => printSlip(slipPath.warranty(claimId), false, size)
