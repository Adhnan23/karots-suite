import { createFileRoute, redirect } from '@tanstack/react-router'
import { meQuery } from '@/api/session'
import { UsersPage } from '@/features/people/users/UsersPage'

export const Route = createFileRoute('/_authed/admin/users')({
  beforeLoad: async ({ context }) => {
    if ((await context.queryClient.ensureQueryData(meQuery)).user.role !== 'admin') throw redirect({ to: '/admin' })
  },
  component: UsersPage,
})
