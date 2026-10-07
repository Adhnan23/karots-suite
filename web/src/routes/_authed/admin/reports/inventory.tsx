import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { InventoryPage } from '@/features/reports/stock/Inventory'
import { searchStr } from '@/lib/search'

const defaults = { category: '', zero: '' }

export const Route = createFileRoute('/_authed/admin/reports/inventory')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ category: searchStr(s.category), zero: searchStr(s.zero) }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: InventoryPage,
})
