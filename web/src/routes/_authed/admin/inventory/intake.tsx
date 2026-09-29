import { createFileRoute, redirect } from '@tanstack/react-router'
import { IntakePage } from '@/features/admin/intake/IntakePage'

export const Route = createFileRoute('/_authed/admin/inventory/intake')({
  beforeLoad: ({ context }) => { if (!context.me.stockTakeEnabled) throw redirect({ to: '/admin/stock' }) },
  component: IntakePage,
})
