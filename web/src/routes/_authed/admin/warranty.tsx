import { createFileRoute } from '@tanstack/react-router'
import { WarrantyPage } from '@/features/sales/WarrantyPage'
import { searchStr } from '@/lib/search'

export const Route = createFileRoute('/_authed/admin/warranty')({
  validateSearch: (s: Record<string, unknown>): { status?: string } => (s.status ? { status: searchStr(s.status) } : {}),
  component: WarrantyPage,
})
