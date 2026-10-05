import { createFileRoute, Outlet, stripSearchParams, useMatchRoute } from '@tanstack/react-router'
import { ReceiptsPage } from '@/features/money/receipts/ReceiptsPage'
import { searchStr } from '@/lib/search'

const defaults = { tab: 'sales', q: '', kind: '', preset: '', from: '', to: '' }

export const Route = createFileRoute('/_authed/admin/receipts')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    tab: searchStr(s.tab) || 'sales', q: searchStr(s.q), kind: searchStr(s.kind), preset: searchStr(s.preset), from: searchStr(s.from), to: searchStr(s.to),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: function Receipts() {
    // /admin/receipts/cash/$id renders the hub itself with that receipt open.
    return useMatchRoute()({ to: '/admin/receipts/cash/$id' }) ? <Outlet /> : <ReceiptsPage />
  },
})
