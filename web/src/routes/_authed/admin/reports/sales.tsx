import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { rangeDefaults, rangeSearch } from '@/features/reports/kit/search'
import { SalesReportPage } from '@/features/reports/sales/SalesReport'
import { searchStr } from '@/lib/search'

const defaults = { ...rangeDefaults, status: '', method: '', fromHour: '', toHour: '' }

export const Route = createFileRoute('/_authed/admin/reports/sales')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({
    ...rangeSearch(s), status: searchStr(s.status), method: searchStr(s.method), fromHour: searchStr(s.fromHour), toHour: searchStr(s.toHour),
  }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: SalesReportPage,
})
