import { createFileRoute, redirect } from '@tanstack/react-router'
import { SuppliersTab } from '@/features/counter/suppliers/SuppliersTab'
import { configQuery } from '@/features/till/api'

export const Route = createFileRoute('/_authed/till/suppliers')({
  // ?supplier=&job=order: opened from Low stock's Order these (lines in prefill.ts)
  validateSearch: (s: Record<string, unknown>): { supplier?: number; job?: 'order' } => ({
    supplier: Number(s.supplier) || undefined, job: s.job === 'order' ? 'order' : undefined,
  }),
  // Hidden from the tab bar without the right; a typed URL goes back to Sell too.
  beforeLoad: async ({ context }) => {
    if (!(await context.queryClient.ensureQueryData(configQuery)).canHandleSuppliers) throw redirect({ to: '/till' })
  },
  component: SuppliersTab,
})
