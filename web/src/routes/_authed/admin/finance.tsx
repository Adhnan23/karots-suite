import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { FinancePage } from '@/features/money/finance/FinancePage'
import { searchStr } from '@/lib/search'

const defaults = { tab: 'overview' as 'overview' | 'profit' | 'cash', preset: 'this-month', from: '', to: '' }

export const Route = createFileRoute('/_authed/admin/finance')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    tab: s.tab === 'profit' || s.tab === 'cash' ? s.tab : 'overview',
    preset: s.preset === undefined ? 'this-month' : searchStr(s.preset), from: searchStr(s.from), to: searchStr(s.to),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: FinancePage,
})
