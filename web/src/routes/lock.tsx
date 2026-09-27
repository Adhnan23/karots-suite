import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { meQuery, redirectFor } from '@/api/session'
import { queryClient } from '@/api/query-client'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { PinForm } from '@/features/auth/PinForm'
import { ModeToggle } from '@/theme/ModeToggle'

export const Route = createFileRoute('/lock')({
  beforeLoad: async ({ context }) => {
    let me
    try {
      me = await context.queryClient.ensureQueryData(meQuery)
    } catch (err) {
      throw redirect({ to: redirectFor(err) ?? '/login' })
    }
    if (!me.locked) throw redirect({ to: '/' })
    return { me }
  },
  component: Lock,
})

function Lock() {
  const { me } = Route.useRouteContext()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  const handleUnlock = async (phone: string, pin: string) => {
    setError(null)
    try {
      await api('/auth/unlock', { json: { phone, pin } })
      queryClient.removeQueries({ queryKey: ['me'] })
      navigate({ to: '/' })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Something went wrong')
    }
  }

  const handleSignOut = () => {
    api('/auth/logout', { method: 'POST' })
      .then(() => window.location.assign('/login'))
      .catch((err) => toast.error(err instanceof ApiError ? err.message : 'Sign out failed'))
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <div className="absolute top-4 right-4">
        <ModeToggle />
      </div>
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="font-display text-xl">Locked — {me.user.name}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <PinForm onSubmit={handleUnlock} submitLabel="Unlock" error={error} />
          <Button type="button" variant="link" onClick={handleSignOut}>
            Sign out
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
