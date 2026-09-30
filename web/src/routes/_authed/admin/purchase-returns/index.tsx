import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { ReturnsPage } from '@/features/purchasing/returns/ReturnsPage'

const defaults = { supplier: 0, page: 1 }

export const Route = createFileRoute('/_authed/admin/purchase-returns/')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ supplier: Number(s.supplier) || 0, page: Math.max(1, Number(s.page) || 1) }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: ReturnsPage,
})
