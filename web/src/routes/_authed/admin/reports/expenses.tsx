import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { ExpensesReportPage } from '@/features/reports/money/Expenses'

export const Route = createFileRoute('/_authed/admin/reports/expenses')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof rangeDefaults> => rangeSearch(s),
  search: { middlewares: [stripSearchParams(rangeDefaults)] },
  component: ExpensesReportPage,
})
