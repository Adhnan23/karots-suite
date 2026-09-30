import { createFileRoute } from '@tanstack/react-router'
import { PurchasePage } from '@/features/purchasing/purchases/PurchasePage'

export const Route = createFileRoute('/_authed/admin/purchases/$id/')({
  component: function Purchase() {
    return <PurchasePage id={Number(Route.useParams().id)} />
  },
})
