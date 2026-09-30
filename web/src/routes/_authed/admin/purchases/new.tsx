import { createFileRoute } from '@tanstack/react-router'
import { PurchaseEntry } from '@/features/purchasing/purchases/PurchaseEntry'

export const Route = createFileRoute('/_authed/admin/purchases/new')({
  validateSearch: (s: Record<string, unknown>): { receive?: boolean; supplier?: number } => ({
    receive: s.receive === true || s.receive === 'true' || undefined,
    supplier: Number(s.supplier) || undefined,
  }),
  component: function NewPurchase() {
    const s = Route.useSearch()
    return <PurchaseEntry key={s.receive ? 'r' : 'o'} mode={s.receive ? 'receive-now' : 'order'} supplierId={s.supplier ?? 0} />
  },
})
