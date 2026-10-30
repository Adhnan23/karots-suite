import QRCode from 'qrcode'
import { Smartphone } from 'lucide-react'
import { useEffect, useState } from 'react'
import { deviceId } from '@/lib/device'
import { BASE } from '../api'
import { CARD, Screen } from '../bits'
import { savePhone, type PhoneId } from '../store/phone'

const PENDING = 'ks-pocket-pairing'

function randomSecret() {
  const b = crypto.getRandomValues(new Uint8Array(32))
  return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')
}

/** First run: name the phone, show its code; an admin approves it in Phones. */
export function Pair({ onPaired }: { onPaired: (p: PhoneId) => void }) {
  const [name, setName] = useState('')
  const [pending, setPending] = useState<(PhoneId & { code: string }) | null>(() => {
    try { return JSON.parse(localStorage.getItem(PENDING) ?? 'null') } catch { return null }
  })
  const [error, setError] = useState('')
  const [qr, setQr] = useState('')

  useEffect(() => {
    if (!pending) return
    void QRCode.toDataURL(pending.code, { margin: 1, width: 220 }).then(setQr)
    const poll = async () => {
      const res = await fetch(`${BASE}/pair/status`, { headers: { 'X-Ks-Phone': pending.secret } }).catch(() => null)
      if (!res?.ok) return
      const st = (await res.json()) as { status: string }
      if (st.status === 'active') {
        const p = { secret: pending.secret, name: pending.name, device: pending.device }
        await savePhone(p)
        localStorage.removeItem(PENDING)
        onPaired(p)
      } else if (st.status === 'blocked') {
        setError('This phone was blocked by the shop.')
      }
    }
    void poll()
    const id = setInterval(poll, 3000)
    return () => clearInterval(id)
  }, [pending, onPaired])

  async function start() {
    setError('')
    const p = { name: name.trim(), device: deviceId() || randomSecret().slice(0, 16), secret: randomSecret() }
    if (!p.name) return setError('Give this phone a name, like “Back store 1”.')
    const res = await fetch(`${BASE}/pair`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(p) }).catch(() => null)
    if (!res) return setError('The shop PC isn’t reachable. Check the wifi and try again.')
    const body = await res.json().catch(() => null)
    if (!res.ok) return setError(body?.error?.message ?? 'Pairing failed')
    const next = { ...p, code: body.code as string }
    localStorage.setItem(PENDING, JSON.stringify(next))
    setPending(next)
  }

  return (
    <Screen>
      <div className="m-3 rounded-[var(--radius)] bg-hero bg-[image:var(--hero-art)] p-5 text-hero-foreground">
        <Smartphone size={26} />
        <h1 className="mt-2 font-display text-2xl font-extrabold">Pocket Cashier</h1>
        <p className="text-sm text-hero-muted">{pending ? 'Ask an admin to approve this phone.' : 'Pair this phone with the shop once.'}</p>
      </div>
      {pending ? (
        <div className="flex flex-col items-center gap-3 px-6 py-4 text-center">
          <p className="text-sm text-muted-foreground">Admin → Phones → Approve, then type or scan:</p>
          <b aria-label="Pairing code" className="font-display text-5xl tracking-[0.2em] tabular-nums">{pending.code}</b>
          {qr && <img src={qr} alt="Pairing code as a QR" className={`${CARD} size-44 bg-paper p-2`} />}
          <p className="text-[12.5px] text-faint">“{pending.name}” · waiting for approval…</p>
          <button type="button" onClick={() => { localStorage.removeItem(PENDING); setPending(null) }} className="text-sm text-muted-foreground underline">Start again</button>
        </div>
      ) : (
        <form className="flex flex-col gap-3 px-4 py-4" onSubmit={(e) => { e.preventDefault(); void start() }}>
          <label htmlFor="pc-name" className="text-xs font-bold text-muted-foreground">This phone’s name</label>
          <input id="pc-name" autoComplete="off" value={name} onChange={(e) => setName(e.target.value)} maxLength={40} placeholder="Back store 1"
            className={`${CARD} h-12 px-3 text-lg font-semibold outline-none focus:border-brand`} />
          <button type="submit" className="h-12 rounded-[var(--radius)] bg-brand font-extrabold text-brand-foreground shadow-sm">Pair this phone</button>
        </form>
      )}
      {error && <p role="alert" className="px-4 text-center text-sm font-semibold text-destructive">{error}</p>}
    </Screen>
  )
}
