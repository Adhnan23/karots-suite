import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { ProductSalesPage } from '@/features/reports/products/ProductSales'
import { searchStr } from '@/lib/search'

const defaults = { ...rangeDefaults, product: 0, grain: '' }

export const Route = createFileRoute('/_authed/admin/reports/product-sales')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ ...rangeSearch(s), product: Number(s.product) || 0, grain: searchStr(s.grain) }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: ProductSalesPage,
})
