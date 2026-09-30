import { createFileRoute } from '@tanstack/react-router'
import { CustomerPage } from '@/features/people/customers/CustomerPage'

export const Route = createFileRoute('/_authed/admin/customers/$id')({
  component: function Customer() {
    return <CustomerPage id={Number(Route.useParams().id)} />
  },
})
