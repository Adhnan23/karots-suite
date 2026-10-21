import { createFileRoute, redirect } from '@tanstack/react-router'
import { LowStockTab } from '@/features/counter/lowstock/LowStockTab'
import { configQuery } from '@/features/till/api'

export const Route = createFileRoute('/_authed/till/low-stock')({
  // Hidden from the tab bar without the right; a typed URL goes back to Sell too.
  beforeLoad: async ({ context }) => {
    if (!(await context.queryClient.ensureQueryData(configQuery)).canSeeLowStock) throw redirect({ to: '/till' })
  },
  component: LowStockTab,
})
