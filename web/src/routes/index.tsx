import { createFileRoute, redirect } from '@tanstack/react-router'
import { meQuery, redirectFor } from '@/api/session'

export const Route = createFileRoute('/')({
  beforeLoad: async ({ context }) => {
    let me
    try {
      me = await context.queryClient.ensureQueryData(meQuery)
    } catch (err) {
      throw redirect({ to: redirectFor(err) ?? '/login' })
    }
    throw redirect({ to: me.user.role === 'cashier' ? '/till' : '/admin' })
  },
})
