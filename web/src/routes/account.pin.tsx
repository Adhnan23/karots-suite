import { createFileRoute, Link, redirect, useNavigate } from '@tanstack/react-router'
import { useRef, useState } from 'react'
import { api, ApiError } from '@/api/client'
import { meQuery, redirectFor } from '@/api/session'
import { queryClient } from '@/api/query-client'
import { PinField } from '@/features/auth/PinField'
import { PinPad, type PinKey } from '@/features/auth/PinPad'
import { ModeToggle } from '@/theme/ModeToggle'

export const Route = createFileRoute('/account/pin')({
  beforeLoad: async ({ context }) => {
    let me
    try {
      me = await context.queryClient.ensureQueryData(meQuery)
    } catch (err) {
      throw redirect({ to: redirectFor(err) ?? '/login' })
    }
    return { me }
  },
  component: ChangePin,
})

type Field = 'current' | 'new' | 'confirm'

function ChangePin() {
  const { me } = Route.useRouteContext()
  const navigate = useNavigate()
  const [currentPin, setCurrentPin] = useState('')
  const [newPin, setNewPin] = useState('')
  const [confirmPin, setConfirmPin] = useState('')
  const [target, setTarget] = useState<Field>('current')
  const [error, setError] = useState<string | null>(null)
  const [shake, setShake] = useState(false)
  const [pending, setPending] = useState(false)
  const currentRef = useRef<HTMLInputElement>(null)
  const newRef = useRef<HTMLInputElement>(null)
  const confirmRef = useRef<HTMLInputElement>(null)

  const setters = { current: setCurrentPin, new: setNewPin, confirm: setConfirmPin }
  const refs = { current: currentRef, new: newRef, confirm: confirmRef }

  const handlePadKey = (k: PinKey) => {
    const apply = (v: string) => (k === 'clear' ? '' : k === 'back' ? v.slice(0, -1) : (v + k).slice(0, 6))
    setters[target]((v) => apply(v))
  }

  const focusNext = (field: Field) => {
    setTarget(field)
    refs[field].current?.focus()
  }

  const handleSubmit = async () => {
    setPending(true)
    setError(null)
    try {
      await api('/auth/pin', { json: { currentPin, newPin, confirmPin } })
      queryClient.removeQueries({ queryKey: ['me'] })
      navigate({ to: '/' })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Something went wrong')
      setShake(true)
      setTimeout(() => setShake(false), 350)
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <div className="absolute top-4 right-4">
        <ModeToggle />
      </div>
      <form
        autoComplete="off"
        onSubmit={(e) => {
          e.preventDefault()
          handleSubmit()
        }}
        className="flex w-full max-w-[360px] flex-col gap-5 rounded-[var(--radius)] border border-border bg-card p-6 shadow-[var(--shadow)]"
      >
        <div className="flex flex-col gap-1">
          <h2 className="font-display text-2xl font-bold tracking-tight">
            {me.mustChangePin ? 'Choose a new PIN to continue' : 'Change PIN'}
          </h2>
          <p className="text-sm text-muted-foreground">Enter your current PIN, then your new one twice.</p>
        </div>

        <div className="flex flex-col gap-1.5">
          <label htmlFor="current-pin" className="text-xs font-bold text-muted-foreground">
            Current PIN
          </label>
          <PinField
            id="current-pin"
            inputRef={currentRef}
            value={currentPin}
            onChange={setCurrentPin}
            onFocus={() => setTarget('current')}
            onEnter={() => focusNext('new')}
            shake={shake}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="new-pin" className="text-xs font-bold text-muted-foreground">
            New PIN
          </label>
          <PinField
            id="new-pin"
            inputRef={newRef}
            value={newPin}
            onChange={setNewPin}
            onFocus={() => setTarget('new')}
            onEnter={() => focusNext('confirm')}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="confirm-pin" className="text-xs font-bold text-muted-foreground">
            Confirm new PIN
          </label>
          <PinField
            id="confirm-pin"
            inputRef={confirmRef}
            value={confirmPin}
            onChange={setConfirmPin}
            onFocus={() => setTarget('confirm')}
            onEnter={handleSubmit}
          />
        </div>

        <p role="alert" className="-my-2 min-h-[21px] text-sm font-semibold text-destructive">
          {error}
        </p>

        <PinPad onKey={handlePadKey} />

        <button
          type="submit"
          disabled={pending}
          className="h-14 rounded-[var(--radius-sm)] bg-brand font-semibold text-brand-foreground shadow-[var(--glow)] disabled:opacity-60"
        >
          Save
        </button>
        {!me.mustChangePin && (
          <Link to="/" className="text-center text-sm text-muted-foreground hover:underline">
            Cancel
          </Link>
        )}
      </form>
    </div>
  )
}
