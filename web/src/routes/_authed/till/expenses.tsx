import { createFileRoute, redirect } from '@tanstack/react-router'
import { ExpensesTab } from '@/features/counter/expenses/ExpensesTab'
import { expensesQuery } from '@/features/counter/queries'
import { configQuery } from '@/features/till/api'

export const Route = createFileRoute('/_authed/till/expenses')({
  // Hidden from the tab bar without the right; a typed URL goes back to Sell too.
  beforeLoad: async ({ context }) => {
    if (!(await context.queryClient.ensureQueryData(configQuery)).canHandleSuppliers) throw redirect({ to: '/till' })
  },
  loader: ({ context }) => context.queryClient.ensureQueryData(expensesQuery),
  component: ExpensesTab,
})
