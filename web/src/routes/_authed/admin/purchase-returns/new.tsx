import { createFileRoute } from '@tanstack/react-router'
import { ReturnEntry } from '@/features/purchasing/returns/ReturnEntry'

export const Route = createFileRoute('/_authed/admin/purchase-returns/new')({
  validateSearch: (s: Record<string, unknown>): { supplier?: number; purchase?: number } => ({
    supplier: Number(s.supplier) || undefined,
    purchase: Number(s.purchase) || undefined,
  }),
  component: function NewReturn() {
    const s = Route.useSearch()
    return <ReturnEntry supplierId={s.supplier ?? 0} purchaseId={s.purchase ?? 0} />
  },
})
