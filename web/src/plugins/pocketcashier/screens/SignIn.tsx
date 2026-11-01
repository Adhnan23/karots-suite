import { useEffect, useState } from 'react'
import { api, ApiError } from '@/api/client'
import type { CopyPerson, LoginResponse, TillCopy } from '@/api/types.gen'
import { PinPad, type PinKey } from '@/features/auth/PinPad'
import { cn } from '@/lib/utils'
import { ago, initials, LivePill, Screen } from '../bits'
import { checkPin } from '../pin'
import type { Who } from '../session'
import type { PhoneId } from '../store/phone'
import type { SyncState } from '../sync'

const ADMIN = -1

/** Tap your name, PIN. Online it signs in on the server (an admin goes to the
 * admin panel); offline it checks the PIN against the copy — cashiers and
 * managers only, admins never have a hash on the phone. */
export function SignIn({ copy, sync, phone, onSignedIn }: { copy: TillCopy | null; sync: SyncState; phone: PhoneId; onSignedIn: (w: Who) => void }) {
  const people = copy?.people ?? []
  const [pick, setPick] = useState<number | null>(people.length === 1 ? people[0].id : null)
  const [adminPhone, setAdminPhone] = useState('')
  const [pin, setPin] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 15_000)
    return () => clearInterval(id)
  }, [])

  async function online(phoneNo: string): Promise<'ok' | 'down' | string> {
    try {
      const r = await api<LoginResponse>('/auth/login', { json: { phone: phoneNo, pin, deviceLabel: phone.name } })
      if (r.user.role === 'admin') {
        window.location.assign('/admin')
        return 'ok'
      }
      onSignedIn({ id: r.user.id, name: r.user.name, role: r.user.role })
      return 'ok'
    } catch (e) {
      if (e instanceof ApiError) return e.message
      return 'down'
    }
  }

  async function submit(p: string) {
    if (busy || pick === null || p.length < 4) return
    setBusy(true)
    setError('')
    const person: CopyPerson | undefined = people.find((x) => x.id === pick)
    let res: string = 'down'
    if (sync.live || pick === ADMIN) res = await online(pick === ADMIN ? adminPhone.trim() : person?.phone ?? '')
    if (res === 'down' && person) {
      res = (await checkPin(person, p)) ? 'ok' : 'Wrong PIN'
      if (res === 'ok') onSignedIn({ id: person.id, name: person.name, role: person.role })
    } else if (res === 'down') {
      res = 'The PC isn’t reachable — an admin signs in only while it is.'
    }
    if (res !== 'ok') {
      setError(res)
      setPin('')
    }
    setBusy(false)
  }

  const key = (k: PinKey) => setPin((v) => (k === 'clear' ? '' : k === 'back' ? v.slice(0, -1) : (v + k).slice(0, 6)))
  // First names, unless two people share one (then their full names).
  const first = (n: string) => n.split(' ')[0]
  const tiles = [...people.map((p) => ({ id: p.id, name: people.filter((q) => first(q.name) === first(p.name)).length > 1 ? p.name : first(p.name) })),
    ...(sync.live ? [{ id: ADMIN, name: 'Admin' }] : [])]

  return (
    <Screen>
      <div className="flex items-center gap-2 px-3 pt-2 pb-2"><LivePill live={sync.live} label={sync.live ? 'Connected to shop' : undefined} /></div>
      <div className="mx-3 rounded-[var(--radius)] bg-hero bg-[image:var(--hero-art)] p-4 text-hero-foreground">
        <div className="font-display text-[46px] leading-none font-extrabold tracking-tight tabular-nums">{now.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}</div>
        <div className="mt-1 text-[12.5px] text-hero-muted">{now.toLocaleDateString([], { weekday: 'long', day: 'numeric', month: 'long' })}{copy?.shop.name ? ` · ${copy.shop.name}` : ''}</div>
      </div>
      <div role="radiogroup" aria-label="Who are you?" className="flex gap-2 overflow-x-auto px-3 py-3">
        {tiles.map((t) => (
          <button key={t.id} type="button" role="radio" aria-checked={pick === t.id} onClick={() => { setPick(t.id); setPin(''); setError('') }}
            className={cn('flex min-w-[56px] flex-col items-center gap-1 text-[11.5px] font-semibold', pick === t.id ? 'text-foreground' : 'text-muted-foreground')}>
            <span className={cn('grid size-[42px] place-items-center rounded-full border-2 text-sm font-bold text-foreground',
              pick === t.id ? 'border-brand bg-brand-soft' : 'border-transparent bg-muted')}>{t.id === ADMIN ? 'AD' : initials(t.name)}</span>
            {t.name}
          </button>
        ))}
        {!tiles.length && <p className="text-sm text-muted-foreground">No one to sign in yet — connect to the shop wifi once to get the copy.</p>}
      </div>
      <form autoComplete="off" className="flex flex-1 flex-col gap-2 px-5" onSubmit={(e) => { e.preventDefault(); void submit(pin) }}>
        {pick === ADMIN && (
          <input aria-label="Admin phone" name="ks-ph" inputMode="tel" autoComplete="off" placeholder="Admin phone number" value={adminPhone}
            onChange={(e) => setAdminPhone(e.target.value)} className="h-11 rounded-[var(--radius-sm)] border border-border bg-card px-3 font-semibold outline-none focus:border-brand" />
        )}
        <div aria-label="PIN" className="my-1 flex justify-center gap-3">
          {[0, 1, 2, 3, 4, 5].slice(0, Math.max(4, pin.length)).map((i) => (
            <span key={i} className={cn('size-3 rounded-full border-2', i < pin.length ? 'border-foreground bg-foreground' : 'border-faint')} />
          ))}
        </div>
        <p role="alert" className="min-h-5 text-center text-sm font-semibold text-destructive">{error}</p>
        {pick !== null && <PinPad onKey={key} />}
        {pick !== null && (
          <button type="submit" disabled={busy || pin.length < 4} className="h-12 rounded-[var(--radius)] bg-brand font-extrabold text-brand-foreground shadow-sm disabled:opacity-50">
            {busy ? 'Signing in…' : 'Sign in'}
          </button>
        )}
      </form>
      <p className="mt-auto px-4 py-3 text-center text-[11.5px] text-muted-foreground">
        Copy updated <b>{ago(sync.copyAt)}</b> · Phone “{phone.name}”
      </p>
    </Screen>
  )
}
