import { createFileRoute } from '@tanstack/react-router'
import { PurchaseEntry } from '@/features/purchasing/purchases/PurchaseEntry'

export const Route = createFileRoute('/_authed/admin/purchases/$id/edit')({
  component: function PurchaseEdit() {
    return <PurchaseEntry mode="edit" id={Number(Route.useParams().id)} />
  },
})
