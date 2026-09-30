import { createFileRoute } from '@tanstack/react-router'
import { SupplierPage } from '@/features/purchasing/suppliers/SupplierPage'

export const Route = createFileRoute('/_authed/admin/suppliers/$id')({
  component: function Supplier() {
    return <SupplierPage id={Number(Route.useParams().id)} />
  },
})
