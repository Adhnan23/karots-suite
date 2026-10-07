import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { PeakHoursPage } from '@/features/reports/sales/PeakHours'

export const Route = createFileRoute('/_authed/admin/reports/peak-hours')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof rangeDefaults> => rangeSearch(s),
  search: { middlewares: [stripSearchParams(rangeDefaults)] },
  component: PeakHoursPage,
})
