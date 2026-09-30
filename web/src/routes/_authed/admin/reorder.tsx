import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { ReorderPage } from '@/features/purchasing/reorder/ReorderPage'
import { searchStr } from '@/lib/search'

const defaults = { q: '', category: 0, supplier: 0, page: 1 }

export const Route = createFileRoute('/_authed/admin/reorder')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    q: searchStr(s.q),
    category: Number(s.category) || 0,
    supplier: Number(s.supplier) || 0,
    page: Math.max(1, Number(s.page) || 1),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: ReorderPage,
})
