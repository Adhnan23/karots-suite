import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { AdminShell } from '@/theme/AdminShell'

export const Route = createFileRoute('/_authed/admin')({
  beforeLoad: ({ context }) => {
    if (context.me.user.role === 'cashier') throw redirect({ to: '/till' })
  },
  component: AdminLayout,
})

function AdminLayout() {
  const { me } = Route.useRouteContext()
  return (
    <AdminShell me={me}>
      <Outlet />
    </AdminShell>
  )
}
