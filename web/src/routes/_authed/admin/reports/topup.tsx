import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { TopupPage } from '@/features/reports/stock/Topup'

const defaults = { category: 0 }

export const Route = createFileRoute('/_authed/admin/reports/topup')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ category: Number(s.category) || 0 }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: TopupPage,
})
