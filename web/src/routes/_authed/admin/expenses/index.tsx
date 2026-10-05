import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { ExpensesPage } from '@/features/money/expenses/ExpensesPage'
import { searchStr } from '@/lib/search'

const defaults = { preset: 'this-month', from: '', to: '', category: '', q: '' }

export const Route = createFileRoute('/_authed/admin/expenses/')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    preset: s.preset === undefined ? 'this-month' : searchStr(s.preset), from: searchStr(s.from), to: searchStr(s.to), category: searchStr(s.category), q: searchStr(s.q),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: ExpensesPage,
})
