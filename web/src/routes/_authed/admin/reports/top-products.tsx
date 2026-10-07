import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { TopProductsPage } from '@/features/reports/products/TopProducts'

const defaults = { ...rangeDefaults, order: 'revenue' }

export const Route = createFileRoute('/_authed/admin/reports/top-products')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ ...rangeSearch(s), order: s.order === 'qty' || s.order === 'profit' ? s.order : 'revenue' }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: TopProductsPage,
})
