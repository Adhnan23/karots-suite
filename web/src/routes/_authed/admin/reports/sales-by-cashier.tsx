import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { CashiersPage } from '@/features/reports/products/Cashiers'

export const Route = createFileRoute('/_authed/admin/reports/sales-by-cashier')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof rangeDefaults> => rangeSearch(s),
  search: { middlewares: [stripSearchParams(rangeDefaults)] },
  component: CashiersPage,
})
