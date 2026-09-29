import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { saleQuery } from '@/features/counter/queries'
import { SalePaper } from '@/features/counter/receipts/Paper'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { configQuery } from '@/features/till/api'

/** Browser view of a sale receipt, for A4 / PDF (karots-pos /cashier/receipt/:id). */
export const Route = createFileRoute('/_authed/print/receipt/$id')({
  loader: ({ context, params }) =>
    Promise.all([context.queryClient.ensureQueryData(configQuery), context.queryClient.ensureQueryData(saleQuery(Number(params.id)))]),
  component: function ReceiptPrint() {
    const { id } = Route.useParams()
    const cfg = useSuspenseQuery(configQuery).data
    const sale = useSuspenseQuery(saleQuery(Number(id))).data
    return (
      <PrintFrame title={`Receipt ${sale.receiptNo}`}>
        <SalePaper s={sale} shop={cfg.shop} sym={cfg.symbol} />
      </PrintFrame>
    )
  },
})
