import type { LabelRequest } from '@/api/types.gen'
import type { LabelKind } from '@/features/print/labels/kit'
import type { Job } from '../types'
import { ClassicTicket } from './classic'
import { JobTicket, jobFits } from './job'

/** What a ticket label shows: received / readyBy as ISO dates. */
export type TicketLabel = { ticketNo: string; code: string; device: string; customer: string; phone: string; received: string; readyBy?: string }

export const ticketOf = (j: Job): TicketLabel => ({
  ticketNo: j.ticketNo, code: j.ticketCode, device: j.deviceModel || j.repairType, customer: j.customerName, phone: j.customerPhone,
  received: j.createdAt, readyBy: j.promisedDate || undefined,
})

/** Classic's request: its scan code, ticket + device on top, the customer under. */
export const ticketRequest = (t: TicketLabel): LabelRequest => ({
  productId: 0, code: t.code, format: 'CODE128', top: `${t.ticketNo} · ${t.device}`, bottom: `${t.customer} ${t.phone}`.trim(), count: 1, size: '',
})

/** Repairs' ticket sticker as a label kind (Image mode). */
export const TICKET_KIND: LabelKind<TicketLabel> = {
  kind: 'repairs.ticket',
  label: 'Repair ticket',
  sample: { ticketNo: 'R-0142', code: 'RPR000142', device: 'Samsung A12', customer: 'Kamal', phone: '077 123 4567', received: '2026-10-26T09:00:00+05:30', readyBy: '2026-10-28' },
  styles: [
    { key: 'classic', label: 'Classic', Template: ClassicTicket },
    { key: 'job', label: 'Job', Template: JobTicket, fits: jobFits },
  ],
}
