import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { CountSheetPage } from '@/features/reports/stock/CountSheetPage'
import { searchStr } from '@/lib/search'

const defaults = { category: '', supplier: '' }

export const Route = createFileRoute('/_authed/admin/reports/count-sheet')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ category: searchStr(s.category), supplier: searchStr(s.supplier) }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: CountSheetPage,
})
