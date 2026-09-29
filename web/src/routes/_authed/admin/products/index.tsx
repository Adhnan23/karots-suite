import { searchStr } from '@/lib/search'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { ProductsPage } from '@/features/admin/products/ProductsPage'

const defaults = { q: '', category: 0, low: false, disabled: false, page: 1 }

export const Route = createFileRoute('/_authed/admin/products/')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    q: searchStr(s.q),
    category: Number(s.category) || 0,
    low: s.low === true || s.low === 'true',
    disabled: s.disabled === true || s.disabled === 'true',
    page: Math.max(1, Number(s.page) || 1),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: ProductsPage,
})
