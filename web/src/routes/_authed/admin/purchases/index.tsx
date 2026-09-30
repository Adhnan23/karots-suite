import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { PurchasesPage } from '@/features/purchasing/purchases/PurchasesPage'
import { searchStr } from '@/lib/search'

const defaults = { tab: 'orders' as 'orders' | 'received', q: '', supplier: 0, unpaid: false, page: 1 }

export const Route = createFileRoute('/_authed/admin/purchases/')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    tab: s.tab === 'received' ? 'received' : 'orders',
    q: searchStr(s.q),
    supplier: Number(s.supplier) || 0,
    unpaid: s.unpaid === true || s.unpaid === 'true',
    page: Math.max(1, Number(s.page) || 1),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: PurchasesPage,
})
