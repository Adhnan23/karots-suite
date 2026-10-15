import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { meQuery, redirectFor } from '@/api/session'
import { queryClient } from '@/api/query-client'
import { LoginHero } from '@/features/auth/LoginHero'
import { PinField } from '@/features/auth/PinField'
import { PinPad, type PinKey } from '@/features/auth/PinPad'
import { loadCart } from '@/features/till/cart/persist'
import { qtyText } from '@/lib/money'
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

const LOCKED_AT = 'ks-locked-at'

/** When this screen was locked: noted the first time the lock page shows (a reload
 * keeps it), forgotten on unlock or sign-out. */
function useLockedFor() {
  const [at] = useState(() => {
    try {
      const v = Number(sessionStorage.getItem(LOCKED_AT))
      if (v) return v
      sessionStorage.setItem(LOCKED_AT, String(Date.now()))
    } catch { /* storage blocked: just "now" */ }
    return Date.now()
  })
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 30_000)
    return () => clearInterval(id)
  }, [])
  const min = Math.floor((now - at) / 60_000)
  return min < 1 ? 'just now' : min === 1 ? '1 min ago' : min < 60 ? `${min} min ago` : `${Math.floor(min / 60)} h ${min % 60} min ago`
}
const forgetLock = () => { try { sessionStorage.removeItem(LOCKED_AT) } catch { /* ignore */ } }

function Lock() {
  const { me } = Route.useRouteContext()
  const navigate = useNavigate()
  const [phone, setPhone] = useState('')
  const [pin, setPin] = useState('')
  const [target, setTarget] = useState<'phone' | 'pin'>('phone')
  const [error, setError] = useState<string | null>(null)
  const [shake, setShake] = useState(false)
  const phoneRef = useRef<HTMLInputElement>(null)
  const pinRef = useRef<HTMLInputElement>(null)
  const lockedFor = useLockedFor()
  // The locked user's sale in progress (items only — never amounts on this screen).
  const open = loadCart(me.user.id)?.lines.reduce((n, l) => n + l.qty, 0) ?? 0

  const handlePadKey = (k: PinKey) => {
    const apply = (v: string) => (k === 'clear' ? '' : k === 'back' ? v.slice(0, -1) : v + k)
    if (target === 'phone') setPhone(apply)
    else setPin((v) => apply(v).slice(0, 6))
  }

  const handleUnlock = async () => {
    setError(null)
    try {
      await api('/auth/unlock', { json: { phone, pin } })
      forgetLock()
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

  const handleSignOut = () => {
    api('/auth/logout', { method: 'POST' })
      .then(() => { forgetLock(); window.location.assign('/login') })
      .catch((err) => toast.error(err instanceof ApiError ? err.message : 'Sign out failed'))
  }

  return (
    <div className="grid min-h-screen md:grid-cols-[1.15fr_1fr]">
      <LoginHero shopName={me.shopName} />
      <form
        autoComplete="off"
        onSubmit={(e) => {
          e.preventDefault()
          handleUnlock()
        }}
        className="relative flex flex-col justify-center p-6 md:items-center md:p-16"
      >
        <div className="absolute top-4 right-4 md:top-6 md:right-6">
          <ModeToggle />
        </div>
        <div className="flex w-full flex-col gap-5 md:max-w-[440px]">
          <div className="flex flex-col gap-1">
            <h2 className="font-display text-2xl font-bold tracking-tight md:text-3xl">Locked by {me.user.name}</h2>
            <p className="text-muted-foreground">Locked {lockedFor} · enter the phone number and PIN to unlock.</p>
            {open > 0 && (
              <p className="mt-2 self-start rounded-full bg-brand-soft px-3 py-1 text-sm font-semibold">
                🛒 A sale with {qtyText(open)} {open === 1 ? 'item' : 'items'} is waiting
              </p>
            )}
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="lock-phone" className="text-xs font-bold text-muted-foreground">
              Phone
            </label>
            <input
              ref={phoneRef}
              id="lock-phone"
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
            <label htmlFor="lock-pin" className="text-xs font-bold text-muted-foreground">
              PIN
            </label>
            <PinField
              id="lock-pin"
              inputRef={pinRef}
              value={pin}
              onChange={setPin}
              onFocus={() => setTarget('pin')}
              onEnter={handleUnlock}
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
            Unlock
          </button>
          <button type="button" onClick={handleSignOut} className="text-center text-sm text-muted-foreground hover:underline">
            Sign out
          </button>
        </div>
      </form>
    </div>
  )
}
