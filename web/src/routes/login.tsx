import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useRef, useState } from 'react'
import { api, ApiError } from '@/api/client'
import { queryClient } from '@/api/query-client'
import { LoginHero } from '@/features/auth/LoginHero'
import { PinField } from '@/features/auth/PinField'
import { PinPad, type PinKey } from '@/features/auth/PinPad'
import { useKonami } from '@/lib/konami'
import { ModeToggle } from '@/theme/ModeToggle'
import type { LoginInfo, LoginResponse } from '@/api/types.gen'

export const Route = createFileRoute('/login')({
  component: Login,
})

function Login() {
  const navigate = useNavigate()
  const [phone, setPhone] = useState('')
  const [pin, setPin] = useState('')
  const [target, setTarget] = useState<'phone' | 'pin'>('phone')
  const [error, setError] = useState<string | null>(null)
  const [shake, setShake] = useState(false)
  const [reveal, setReveal] = useState(false)
  const phoneRef = useRef<HTMLInputElement>(null)
  const pinRef = useRef<HTMLInputElement>(null)
  const { data: info } = useQuery({ queryKey: ['auth', 'login-info'], queryFn: () => api<LoginInfo>('/auth/login-info') })
  useKonami(() => setReveal(true))

  const handlePadKey = (k: PinKey) => {
    const apply = (v: string) => (k === 'clear' ? '' : k === 'back' ? v.slice(0, -1) : v + k)
    if (target === 'phone') setPhone(apply)
    else setPin((v) => apply(v).slice(0, 6))
  }

  const handleSubmit = async () => {
    setError(null)
    try {
      await api<LoginResponse>('/auth/login', { json: { phone, pin } })
      try { sessionStorage.removeItem('ks-locked-at') } catch { /* ignore */ } // a fresh sign-in starts no lock clock
      queryClient.removeQueries({ queryKey: ['me'] })
      navigate({ to: '/' })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Something went wrong')
      setPin('')
      setShake(true)
      setTimeout(() => setShake(false), 350)
      pinRef.current?.focus()
    }
  }

  return (
    <div className="grid min-h-screen md:grid-cols-[1.15fr_1fr]">
      <LoginHero shopName={info?.shopName || 'Karots Suite'} />
      <form
        autoComplete="off"
        onSubmit={(e) => {
          e.preventDefault()
          handleSubmit()
        }}
        className="relative flex flex-col justify-center p-6 md:items-center md:p-16"
      >
        <div className="absolute top-4 right-4 md:top-6 md:right-6">
          <ModeToggle />
        </div>
        <div className="flex w-full flex-col gap-5 md:max-w-[440px]">
          <div className="flex flex-col gap-1">
            <h2 className="font-display text-2xl font-bold tracking-tight md:text-3xl">Sign in to the counter</h2>
            <p className="text-muted-foreground">Your phone number, then your PIN.</p>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="login-phone" className="text-xs font-bold text-muted-foreground">
              Phone
            </label>
            <input
              ref={phoneRef}
              id="login-phone"
              name="ks-ph"
              type="text"
              inputMode="tel"
              autoComplete="off"
              autoCorrect="off"
              spellCheck={false}
              placeholder="07X XXX XXXX"
              value={phone}
              onFocus={() => setTarget('phone')}
              onChange={(e) => setPhone(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  pinRef.current?.focus()
                }
              }}
              className="h-[62px] rounded-[var(--radius-sm)] border-[1.5px] border-border bg-card px-4 font-display text-2xl font-bold tracking-wide outline-none focus:border-brand"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="login-pin" className="text-xs font-bold text-muted-foreground">
              PIN
            </label>
            <PinField
              id="login-pin"
              inputRef={pinRef}
              value={pin}
              onChange={setPin}
              onFocus={() => setTarget('pin')}
              onEnter={handleSubmit}
              shake={shake}
            />
          </div>
          <p role="alert" className="-my-2 min-h-[21px] text-sm font-semibold text-destructive">
            {error}
          </p>
          <PinPad onKey={handlePadKey} />
          <button
            type="submit"
            className="h-16 rounded-[var(--radius-sm)] bg-brand text-lg font-semibold text-brand-foreground shadow-[var(--glow)]"
          >
            Sign in
          </button>
          {reveal && info && (
            <p className="flex gap-3.5 text-sm text-muted-foreground">
              Install ID <b className="font-mono text-foreground">{info.installId}</b>
              Server time <b className="font-mono text-foreground">{info.serverTime}</b>
            </p>
          )}
        </div>
      </form>
    </div>
  )
}
