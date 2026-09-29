import { createFileRoute } from '@tanstack/react-router'
import { ExpensesTab, expensesQuery } from '@/features/counter/expenses/ExpensesTab'

export const Route = createFileRoute('/_authed/till/expenses')({
  loader: ({ context }) => context.queryClient.ensureQueryData(expensesQuery),
  component: ExpensesTab,
})
