import { createFileRoute, Outlet } from '@tanstack/react-router'
import { useEffect } from 'react'
import { startAutofit } from '@/lib/autofit'
import { TillShell } from '@/theme/TillShell'

export const Route = createFileRoute('/_authed/till')({
  component: TillLayout,
})

function TillLayout() {
  const { me } = Route.useRouteContext()
  useEffect(() => startAutofit(), [])
  return (
    <TillShell me={me}>
      <Outlet />
    </TillShell>
  )
}
