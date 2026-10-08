import { createFileRoute, Outlet } from '@tanstack/react-router'
import { useEffect } from 'react'
import { startAutofit } from '@/lib/autofit'
import { configQuery } from '@/features/till/api'
import { DrawerPill } from '@/features/till/drawer/DrawerPill'
import { SlipAskHost } from '@/features/till/SlipAsk'
import { TillProvider } from '@/features/till/TillProvider'
import { TillShell } from '@/theme/TillShell'

export const Route = createFileRoute('/_authed/till')({
  loader: ({ context }) => context.queryClient.ensureQueryData(configQuery),
  component: TillLayout,
})

function TillLayout() {
  const { me } = Route.useRouteContext()
  const cfg = Route.useLoaderData()
  useEffect(() => startAutofit(), [])
  return (
    <TillProvider userId={me.user.id} saleType={cfg.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail'}>
      <TillShell me={me} config={cfg} status={<DrawerPill />}>
        <Outlet />
        <SlipAskHost />
      </TillShell>
    </TillProvider>
  )
}
