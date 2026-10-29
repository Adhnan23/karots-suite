import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { ChequesPage } from '@/features/money/cheques/ChequesPage'
import { searchStr } from '@/lib/search'

const defaults = { dir: 'in', status: 'pending', preset: '', from: '', to: '', q: '', due: '' }

export const Route = createFileRoute('/_authed/admin/cheques/')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    dir: s.dir === 'out' ? 'out' : 'in', status: s.status === undefined ? 'pending' : searchStr(s.status), preset: searchStr(s.preset),
    from: searchStr(s.from), to: searchStr(s.to), q: searchStr(s.q), due: s.due === '1' || s.due === 1 ? '1' : '',
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: ChequesPage,
})
