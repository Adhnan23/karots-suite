import { createFileRoute } from '@tanstack/react-router'
import { TillScreen } from '@/features/till/TillScreen'

type DrawerMove = 'deposit' | 'withdraw' | 'close'

export const Route = createFileRoute('/_authed/till/')({
  // ?logout=1: sign-out was refused because the till is open — go straight to the count.
  // ?drawer=deposit|withdraw|close: a move picked from the top bar's drawer chip.
  validateSearch: (s: Record<string, unknown>): { logout?: boolean; drawer?: DrawerMove } => ({
    ...(s.logout === true || s.logout === '1' || s.logout === 1 ? { logout: true } : {}),
    ...(s.drawer === 'deposit' || s.drawer === 'withdraw' || s.drawer === 'close' ? { drawer: s.drawer } : {}),
  }),
  component: function TillIndex() {
    const { logout, drawer } = Route.useSearch()
    return <TillScreen logout={!!logout} drawer={drawer} />
  },
})
