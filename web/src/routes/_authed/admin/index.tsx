import { createFileRoute } from '@tanstack/react-router'
import { Dashboard } from '@/features/dashboard/Dashboard'

export const Route = createFileRoute('/_authed/admin/')({
  validateSearch: (s: Record<string, unknown>): { days?: number } => ([7, 30, 90].includes(Number(s.days)) ? { days: Number(s.days) } : {}),
  component: Dashboard,
})
