import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { CategoryProfitPage } from '@/features/reports/money/CategoryProfit'

const defaults = { ...rangeDefaults, category: 0 }

export const Route = createFileRoute('/_authed/admin/reports/profit-by-category')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ ...rangeSearch(s), category: Number(s.category) || 0 }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: CategoryProfitPage,
})
