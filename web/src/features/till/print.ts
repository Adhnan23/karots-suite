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

/** Fire-and-forget: the sale is saved; only the paper is at stake. kick folds the
 * drawer pulse into this ONE printer job (fresh cash sale in auto-print mode). */
export const printSale = (id: number, kick = false) => send(`/till/sales/${id}/print${kick ? '?kick=1' : ''}`)
export const printMoneySlip = (id: number, kick = false) => send(`/till/money-receipts/${id}/print${kick ? '?kick=1' : ''}`)
export const printRefund = (saleId: number, kick = false) => send(`/till/sales/${saleId}/refund-print${kick ? '?kick=1' : ''}`)
export const printDebtSlip = (paymentId: number, kick = false) => send(`/till/customer-payments/${paymentId}/print${kick ? '?kick=1' : ''}`)
