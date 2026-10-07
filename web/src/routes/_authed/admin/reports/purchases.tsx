import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { PurchasesReportPage } from '@/features/reports/people/Purchases'

export const Route = createFileRoute('/_authed/admin/reports/purchases')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof rangeDefaults> => rangeSearch(s),
  search: { middlewares: [stripSearchParams(rangeDefaults)] },
  component: PurchasesReportPage,
})
