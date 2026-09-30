import { createFileRoute } from '@tanstack/react-router'
import { PurchaseEntry } from '@/features/purchasing/purchases/PurchaseEntry'

export const Route = createFileRoute('/_authed/admin/purchases/$id/receive')({
  component: function PurchaseReceive() {
    return <PurchaseEntry mode="receive" id={Number(Route.useParams().id)} />
  },
})
