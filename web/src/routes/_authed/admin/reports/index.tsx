import { createFileRoute } from '@tanstack/react-router'
import { ReportsHub } from '@/features/reports/ReportsHub'

export const Route = createFileRoute('/_authed/admin/reports/')({
  component: function Hub() {
    const { me } = Route.useRouteContext()
    return <ReportsHub me={me} />
  },
})
