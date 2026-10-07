import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { ServiceProfitPage } from '@/features/reports/ops/ServiceProfit'

export const Route = createFileRoute('/_authed/admin/reports/service-profit')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof rangeDefaults> => rangeSearch(s),
  search: { middlewares: [stripSearchParams(rangeDefaults)] },
  component: ServiceProfitPage,
})
