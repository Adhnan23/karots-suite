import { searchStr } from '@/lib/search'
import { createFileRoute, redirect, stripSearchParams } from '@tanstack/react-router'
import { StockTakePage } from '@/features/admin/stock/StockTakePage'

type Search = { q?: string; category?: number; supplier?: number; page?: number }

export const Route = createFileRoute('/_authed/admin/stock/take')({
  // Settings → stock-take & intake off: the page (and its API) is gone, as in karots-pos.
  beforeLoad: ({ context }) => { if (!context.me.stockTakeEnabled) throw redirect({ to: '/admin/stock' }) },
  validateSearch: (s: Record<string, unknown>): Search => ({
    q: searchStr(s.q), category: Number(s.category) || 0, supplier: Number(s.supplier) || 0, page: Math.max(1, Number(s.page) || 1),
  }),
  search: { middlewares: [stripSearchParams({ q: '', category: 0, supplier: 0, page: 1 })] },
  component: StockTakePage,
})
