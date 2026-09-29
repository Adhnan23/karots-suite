import { createFileRoute } from '@tanstack/react-router'
import { ExpensesTab } from '@/features/counter/expenses/ExpensesTab'
import { expensesQuery } from '@/features/counter/queries'

export const Route = createFileRoute('/_authed/till/expenses')({
  loader: ({ context }) => context.queryClient.ensureQueryData(expensesQuery),
  component: ExpensesTab,
})
