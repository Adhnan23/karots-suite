import { createFileRoute } from '@tanstack/react-router'
import { ReturnsTab } from '@/features/counter/returns/ReturnsTab'

export const Route = createFileRoute('/_authed/till/returns')({
  // ?sale=<id> opens Returns on that sale (e.g. a plugin's "return a faulty part").
  validateSearch: (s: Record<string, unknown>): { sale?: number } => (Number(s.sale) > 0 ? { sale: Number(s.sale) } : {}),
  component: ReturnsTabRoute,
})

function ReturnsTabRoute() {
  return <ReturnsTab sale={Route.useSearch().sale} />
}
