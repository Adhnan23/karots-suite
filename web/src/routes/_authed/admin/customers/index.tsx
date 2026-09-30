import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { CustomersPage } from '@/features/people/customers/CustomersPage'
import { searchStr } from '@/lib/search'

type Show = '' | 'owing' | 'advance' | 'over'
const defaults = { q: '', show: '' as Show, disabled: false, page: 1 }

export const Route = createFileRoute('/_authed/admin/customers/')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    q: searchStr(s.q),
    show: s.show === 'owing' || s.show === 'advance' || s.show === 'over' ? s.show : '',
    disabled: s.disabled === true || s.disabled === 'true',
    page: Math.max(1, Number(s.page) || 1),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: CustomersPage,
})
