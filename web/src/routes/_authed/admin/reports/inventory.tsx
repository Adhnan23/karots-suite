import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { InventoryPage } from '@/features/reports/stock/Inventory'
import { searchStr } from '@/lib/search'

const defaults = { category: 0, zero: '' }

export const Route = createFileRoute('/_authed/admin/reports/inventory')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ category: Number(s.category) || 0, zero: searchStr(s.zero) }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: InventoryPage,
})
