import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { CountSheetPage } from '@/features/reports/stock/CountSheetPage'

const defaults = { category: 0, supplier: 0 }

export const Route = createFileRoute('/_authed/admin/reports/count-sheet')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ category: Number(s.category) || 0, supplier: Number(s.supplier) || 0 }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: CountSheetPage,
})
