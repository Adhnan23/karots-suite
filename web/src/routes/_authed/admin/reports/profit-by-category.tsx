import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { CategoryProfitPage } from '@/features/reports/money/CategoryProfit'
import { searchStr } from '@/lib/search'

const defaults = { ...rangeDefaults, category: '' }

export const Route = createFileRoute('/_authed/admin/reports/profit-by-category')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ ...rangeSearch(s), category: searchStr(s.category) }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: CategoryProfitPage,
})
