import { createFileRoute } from '@tanstack/react-router'
import { ReturnPage } from '@/features/purchasing/returns/ReturnPage'

export const Route = createFileRoute('/_authed/admin/purchase-returns/$id')({
  component: function PurchaseReturn() {
    return <ReturnPage id={Number(Route.useParams().id)} />
  },
})
