import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { startAutofit } from '@/lib/autofit'
import { configQuery, drawerQuery } from '@/features/till/api'
import { DrawerPill } from '@/features/till/drawer/DrawerPill'
import { SlipAskHost } from '@/features/till/SlipAsk'
import { TillProvider } from '@/features/till/TillProvider'
import { TillShell } from '@/theme/TillShell'

export const Route = createFileRoute('/_authed/till')({
  loader: ({ context }) => context.queryClient.ensureQueryData(configQuery),
  component: TillLayout,
})

/** A till left open from an earlier day: any other till tab sends the cashier to Sell,
 * where it must be closed first. Checked when moving to a tab only — never mid-task. */
function StaleToSell() {
  const path = useRouterState({ select: (s) => s.location.pathname })
  const d = useQuery(drawerQuery).data
  const navigate = useNavigate()
  const checked = useRef<string | null>(null)
  useEffect(() => {
    if (!d || checked.current === path) return
    checked.current = path
    if (d.summary.stale && path !== '/till') {
      toast.warning('Close the till left open from an earlier day first')
      void navigate({ to: '/till' })
    }
  }, [path, d, navigate])
  return null
}

function TillLayout() {
  const { me } = Route.useRouteContext()
  const cfg = Route.useLoaderData()
  useEffect(() => startAutofit(), [])
  return (
    <TillProvider userId={me.user.id} saleType={cfg.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail'}>
      <TillShell me={me} config={cfg} status={<DrawerPill />}>
        <Outlet />
        <StaleToSell />
        <SlipAskHost />
      </TillShell>
    </TillProvider>
  )
}
