import { createFileRoute } from '@tanstack/react-router'
import { TillScreen } from '@/features/till/TillScreen'

export const Route = createFileRoute('/_authed/till/')({
  // ?logout=1: sign-out was refused because the till is open — go straight to the count.
  validateSearch: (s: Record<string, unknown>): { logout?: boolean } =>
    s.logout === true || s.logout === '1' || s.logout === 1 ? { logout: true } : {},
  component: function TillIndex() {
    const { logout } = Route.useSearch()
    return <TillScreen logout={!!logout} />
  },
})
