import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { CashflowPage } from '@/features/money/cashflow/CashflowPage'
import { searchStr } from '@/lib/search'

const defaults = { preset: 'today', from: '', to: '', place: '', kind: '' }

export const Route = createFileRoute('/_authed/admin/cashflow')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    preset: s.preset === undefined ? 'today' : searchStr(s.preset), from: searchStr(s.from), to: searchStr(s.to), place: searchStr(s.place), kind: searchStr(s.kind),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: CashflowPage,
})
