import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { SalesPage } from '@/features/sales/SalesPage'
import { searchStr } from '@/lib/search'

const defaults = { preset: 'this-month', from: '', to: '', status: '', method: '', q: '' }

export const Route = createFileRoute('/_authed/admin/sales')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    preset: s.preset === undefined ? 'this-month' : searchStr(s.preset), from: searchStr(s.from), to: searchStr(s.to),
    status: searchStr(s.status), method: searchStr(s.method), q: searchStr(s.q),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: SalesPage,
})
