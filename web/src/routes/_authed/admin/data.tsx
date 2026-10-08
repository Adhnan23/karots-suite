import { createFileRoute, redirect } from '@tanstack/react-router'
import { meQuery } from '@/api/session'
import { DataPage } from '@/features/data/DataPage'

export const Route = createFileRoute('/_authed/admin/data')({
  beforeLoad: async ({ context }) => {
    if ((await context.queryClient.ensureQueryData(meQuery)).user.role !== 'admin') throw redirect({ to: '/admin' })
  },
  component: DataPage,
})
