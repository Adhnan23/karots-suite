import { createFileRoute, redirect } from '@tanstack/react-router'
import { SuppliersTab } from '@/features/counter/suppliers/SuppliersTab'
import { configQuery } from '@/features/till/api'

export const Route = createFileRoute('/_authed/till/suppliers')({
  // Hidden from the tab bar without the right; a typed URL goes back to Sell too.
  beforeLoad: async ({ context }) => {
    if (!(await context.queryClient.ensureQueryData(configQuery)).canHandleSuppliers) throw redirect({ to: '/till' })
  },
  component: SuppliersTab,
})
