import { queryOptions, useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { api } from '@/api/client'
import type { SlipData } from '@/api/types.gen'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { slipElement } from '@/features/print/templates'
import { configQuery } from '@/features/till/api'
import { slipPath } from '@/features/till/print'

const slipData = (id: number) =>
  queryOptions({ queryKey: ['slip-data', id], queryFn: () => api<SlipData>(`${slipPath.sale(id)}?data=1`, { method: 'POST' }) })

/** Browser view of a sale receipt, for A4 / PDF (karots-pos /cashier/receipt/:id):
 * the receipt's own template in the shop's style, the same page Image mode prints. */
export const Route = createFileRoute('/_authed/print/receipt/$id')({
  loader: ({ context, params }) =>
    Promise.all([context.queryClient.ensureQueryData(slipData(Number(params.id))), context.queryClient.ensureQueryData(configQuery)]),
  component: function ReceiptPrint() {
    const { id } = Route.useParams()
    const d = useSuspenseQuery(slipData(Number(id))).data
    const style = useSuspenseQuery(configQuery).data.receiptStyle
    return (
      <PrintFrame title={`Receipt ${d.sale?.receiptNo ?? ''}`}>
        <div className="mx-auto w-fit shadow-[var(--shadow)] print:shadow-none">{slipElement(d, style)}</div>
      </PrintFrame>
    )
  },
})
