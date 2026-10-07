import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { TopupPage } from '@/features/reports/stock/Topup'
import { searchStr } from '@/lib/search'

const defaults = { category: '' }

export const Route = createFileRoute('/_authed/admin/reports/topup')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ category: searchStr(s.category) }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: TopupPage,
})
