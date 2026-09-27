import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { api, ApiError } from '@/api/client'
import { queryClient } from '@/api/query-client'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useKonami } from '@/lib/konami'
import { PinForm } from '@/features/auth/PinForm'
import { ModeToggle } from '@/theme/ModeToggle'
import type { LoginInfo, LoginResponse } from '@/api/types.gen'

export const Route = createFileRoute('/login')({
  component: Login,
})

function Login() {
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)
  const [reveal, setReveal] = useState(false)
  const { data: info } = useQuery({ queryKey: ['auth', 'login-info'], queryFn: () => api<LoginInfo>('/auth/login-info') })
  useKonami(() => setReveal(true))

  useEffect(() => {
    if (info) console.info(`POS install ${info.installId} · server time ${info.serverTime}`)
  }, [info])

  const handleSubmit = async (phone: string, pin: string) => {
    setError(null)
    try {
      await api<LoginResponse>('/auth/login', { json: { phone, pin } })
      queryClient.removeQueries({ queryKey: ['me'] })
      navigate({ to: '/' })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Something went wrong')
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <div className="absolute top-4 right-4">
        <ModeToggle />
      </div>
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="font-display text-xl">Karots Suite</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <PinForm onSubmit={handleSubmit} submitLabel="Sign in" error={error} />
          {reveal && info && (
            <div className="rounded-lg bg-muted p-3 text-sm">
              <p className="text-muted-foreground">Install ID</p>
              <p className="font-mono">{info.installId}</p>
              <p className="text-muted-foreground">{info.serverTime}</p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
