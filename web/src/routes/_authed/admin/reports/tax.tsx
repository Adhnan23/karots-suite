import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { TaxPage } from '@/features/reports/money/Tax'

export const Route = createFileRoute('/_authed/admin/reports/tax')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof rangeDefaults> => rangeSearch(s),
  search: { middlewares: [stripSearchParams(rangeDefaults)] },
  component: TaxPage,
})
