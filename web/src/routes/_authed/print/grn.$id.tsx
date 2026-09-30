import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { GrnSheet } from '@/features/purchasing/purchases/PurchaseDocs'
import { purchaseQuery } from '@/features/purchasing/queries'
import { configQuery } from '@/features/till/api'

export const Route = createFileRoute('/_authed/print/grn/$id')({
  loader: ({ context, params }) => Promise.all([context.queryClient.ensureQueryData(configQuery), context.queryClient.ensureQueryData(purchaseQuery(Number(params.id)))]),
  component: function GrnPrint() {
    const cfg = useSuspenseQuery(configQuery).data
    const p = useSuspenseQuery(purchaseQuery(Number(Route.useParams().id))).data
    return <PrintFrame title={`Goods received · ${p.supplierName}`}><GrnSheet p={p} shop={cfg.shop} sym={cfg.symbol} /></PrintFrame>
  },
})
