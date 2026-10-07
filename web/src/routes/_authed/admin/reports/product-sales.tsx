import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { ProductSalesPage } from '@/features/reports/products/ProductSales'
import { searchStr } from '@/lib/search'

const defaults = { ...rangeDefaults, product: '', grain: '' }

export const Route = createFileRoute('/_authed/admin/reports/product-sales')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ ...rangeSearch(s), product: searchStr(s.product), grain: searchStr(s.grain) }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: ProductSalesPage,
})
