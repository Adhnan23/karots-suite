import { createFileRoute, redirect, stripSearchParams } from '@tanstack/react-router'
import { meQuery } from '@/api/session'
import { ActivityPage } from '@/features/activity/ActivityPage'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { searchStr } from '@/lib/search'

const defaults = { ...rangeDefaults, user: 0, source: '', q: '' }

export const Route = createFileRoute('/_authed/admin/activity')({
  beforeLoad: async ({ context }) => {
    if ((await context.queryClient.ensureQueryData(meQuery)).user.role !== 'admin') throw redirect({ to: '/admin' })
  },
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ ...rangeSearch(s), user: Number(s.user) || 0, source: searchStr(s.source), q: searchStr(s.q) }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: ActivityPage,
})
