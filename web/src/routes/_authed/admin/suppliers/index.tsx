import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { SuppliersPage } from '@/features/purchasing/suppliers/SuppliersPage'
import { searchStr } from '@/lib/search'

const defaults = { q: '', show: '' as '' | 'owing' | 'credit', disabled: false, page: 1 }

export const Route = createFileRoute('/_authed/admin/suppliers/')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    q: searchStr(s.q),
    show: s.show === 'owing' || s.show === 'credit' ? s.show : '',
    disabled: s.disabled === true || s.disabled === 'true',
    page: Math.max(1, Number(s.page) || 1),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: SuppliersPage,
})
