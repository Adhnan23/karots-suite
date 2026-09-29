import { searchStr } from '@/lib/search'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { StockPage } from '@/features/admin/stock/StockPage'

type Search = { view?: 'levels' | 'moves'; q?: string; category?: number; low?: boolean; page?: number; product?: number; type?: string; user?: number; preset?: string; from?: string; to?: string }
const str = searchStr

export const Route = createFileRoute('/_authed/admin/stock/')({
  validateSearch: (s: Record<string, unknown>): Search => ({
    view: s.view === 'moves' ? 'moves' : 'levels', q: str(s.q), category: Number(s.category) || 0, low: s.low === true || s.low === 'true',
    page: Math.max(1, Number(s.page) || 1), product: Number(s.product) || 0, type: str(s.type), user: Number(s.user) || 0,
    preset: str(s.preset), from: str(s.from), to: str(s.to),
  }),
  search: { middlewares: [stripSearchParams({ view: 'levels', q: '', category: 0, low: false, page: 1, product: 0, type: '', user: 0, preset: '', from: '', to: '' })] },
  component: StockPage,
})
