import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { zQuery } from '@/features/counter/queries'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { ZPage } from '@/features/counter/receipts/ZPage'
import { configQuery } from '@/features/till/api'

export const Route = createFileRoute('/_authed/print/z/$id')({
  loader: ({ context, params }) =>
    Promise.all([context.queryClient.ensureQueryData(configQuery), context.queryClient.ensureQueryData(zQuery(Number(params.id)))]),
  component: function ZPrint() {
    const { id } = Route.useParams()
    const cfg = useSuspenseQuery(configQuery).data
    const z = useSuspenseQuery(zQuery(Number(id))).data
    return (
      <PrintFrame title={z.closedAt ? 'Day-end (Z) report' : 'Shift so far'}>
        <ZPage z={z} shop={cfg.shop} sym={cfg.symbol} />
      </PrintFrame>
    )
  },
})
