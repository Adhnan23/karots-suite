import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { LockerPage } from '@/features/money/lockers/LockerPage'
import { searchStr } from '@/lib/search'

const defaults = { preset: 'this-month', from: '', to: '' }

export const Route = createFileRoute('/_authed/admin/lockers/$id')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ preset: s.preset === undefined ? 'this-month' : searchStr(s.preset), from: searchStr(s.from), to: searchStr(s.to) }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: LockerPage,
})
