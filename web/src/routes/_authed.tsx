import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { useEffect } from 'react'
import { meQuery, redirectFor } from '@/api/session'
import { applyAppearance } from '@/theme/appearance'
import { AutoLock } from '@/features/auth/AutoLock'
import { KioskExit } from '@/features/kiosk/KioskExit'

export const Route = createFileRoute('/_authed')({
  beforeLoad: async ({ context }) => {
    let me
    try {
      me = await context.queryClient.ensureQueryData(meQuery)
    } catch (err) {
      throw redirect({ to: redirectFor(err) ?? '/login' })
    }
    if (me.locked) throw redirect({ to: '/lock' })
    if (me.mustChangePin) throw redirect({ to: '/account/pin' })
    return { me }
  },
  component: Authed,
})

function Authed() {
  const { me } = Route.useRouteContext()
  useEffect(() => applyAppearance(me.appearance), [me.appearance])
  return (
    <>
      <Outlet />
      <AutoLock minutes={me.lockTimeoutMinutes} />
      {me.flags.canExitKiosk && <KioskExit />}
    </>
  )
}
