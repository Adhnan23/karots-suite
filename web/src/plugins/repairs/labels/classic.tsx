import { queryClient } from '@/api/query-client'
import type { TillConfig } from '@/api/types.gen'
import type { LabelTemplateProps } from '@/features/print/labels/kit'
import { toPriceLabel } from '@/features/print/labels/price'
import { ClassicPrice } from '@/features/print/labels/price/classic'
import { configQuery } from '@/features/till/api'
import { ticketRequest, type TicketLabel } from '.'

/** Today's ticket label: core's Classic barcode label of the ticket request. */
export function ClassicTicket({ d, paper }: LabelTemplateProps<TicketLabel>) {
  const cfg = queryClient.getQueryData(configQuery.queryKey) ?? ({} as TillConfig)
  return <ClassicPrice d={toPriceLabel(ticketRequest(d), cfg)} paper={paper} />
}
