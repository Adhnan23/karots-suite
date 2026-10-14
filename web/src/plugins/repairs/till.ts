import { toast } from 'sonner'
import type { SaleHookCtx, TillApi } from '../slots'
import { getBill } from './api'
import type { Job, LineTag } from './types'

/** A scanned ticket (RPR000012) or a typed ticket number (R-0012). */
export const isTicket = (text: string) => /^(RPR\d{6}|R-\d{4,})$/i.test(text.trim())

export const tagOf = (plugin?: Record<string, unknown>) => plugin?.repairs as LineTag | undefined

/** Put on the bill: the job's customer on the sale, its parts and labour as cart
 * lines tagged with the job, and the deposit pre-filled as a payment row — then
 * the normal Pay screen does the rest. Resolves to an error text, or null. */
export async function putOnBill(till: TillApi, job: Job): Promise<string | null> {
  if (till.lines.some((l) => tagOf(l.plugin)?.job === job.id)) return `${job.ticketNo} is already on the bill`
  const bill = await getBill(job.id)
  const tag: LineTag = { job: job.id, ticket: job.ticketNo }
  if (bill.customerId) till.setCustomer(bill.customerId)
  for (const l of bill.lines) {
    if (l.kind === 'labour') {
      till.addServiceLine({ productId: l.productId, name: l.name, price: Number(l.price), plugin: { repairs: tag } })
      continue
    }
    const discount = l.discount ? { value: l.discount, type: l.discountType || 'fixed' } as const : undefined
    const err = await till.addProduct(l.productId, { units: Number(l.qty), plugin: { repairs: tag }, discount })
    if (err) return `${l.name}: ${err}`
  }
  if (Number(bill.onAccount) > 0) till.addTender('credit', bill.onAccount)
  if (Number(bill.legacyHeld) > 0) till.addTender('deposit', bill.legacyHeld, { job: String(job.id) })
  return null
}

/** After a sale with job lines: the sale itself marked each job collected (its
 * tagged lines, in the sale's transaction) — just say so. */
export function afterSale({ lines }: SaleHookCtx) {
  const tickets = new Set(lines.map((l) => tagOf(l.plugin)?.ticket).filter(Boolean))
  for (const t of tickets) toast.success(`${t} collected`)
  return Promise.resolve()
}
