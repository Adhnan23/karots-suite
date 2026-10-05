import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { TillsPage } from '@/features/money/tills/TillsPage'
import { searchStr } from '@/lib/search'

const defaults = { preset: '', from: '', to: '', user: 0, open: false }

export const Route = createFileRoute('/_authed/admin/tills/')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    preset: searchStr(s.preset), from: searchStr(s.from), to: searchStr(s.to), user: Number(s.user) || 0, open: s.open === true || s.open === 'true',
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: TillsPage,
})
