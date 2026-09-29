import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { orderQuery } from '@/features/counter/queries'
import { OrderSheet } from '@/features/counter/receipts/OrderSheet'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { configQuery } from '@/features/till/api'

export const Route = createFileRoute('/_authed/print/order/$id')({
  loader: ({ context, params }) =>
    Promise.all([context.queryClient.ensureQueryData(configQuery), context.queryClient.ensureQueryData(orderQuery(Number(params.id)))]),
  component: function OrderPrint() {
    const { id } = Route.useParams()
    const cfg = useSuspenseQuery(configQuery).data
    const o = useSuspenseQuery(orderQuery(Number(id))).data
    return (
      <PrintFrame title={`Order #${o.id} · ${o.supplierName}`}>
        <OrderSheet o={o} shop={cfg.shop} />
      </PrintFrame>
    )
  },
})
