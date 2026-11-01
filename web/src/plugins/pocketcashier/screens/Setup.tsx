import { CheckCircle2, Download, Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '@/api/client'
import type { LoginInfo } from '@/api/types.gen'
import { applyPalette } from '@/theme/appearance'
import { BASE } from '../api'
import { CARD, Screen } from '../bits'
import { phoneKind, steps, watchTrust } from '../setupSteps'

const HTTPS_PORT = 8443 // the plugin's Port (pocketcashier.go)

/** Opened on plain HTTP from the Phones QR: this phone's exact steps to trust the
 * shop's certificate. The app link appears only once the phone trusts it, so it
 * never meets a certificate warning. */
export function Setup() {
  const [shop, setShop] = useState('')
  const [trusted, setTrusted] = useState(false)
  const kind = phoneKind(navigator.userAgent)
  const app = `https://${location.hostname}:${HTTPS_PORT}/x/pocketcashier/`
  useEffect(() => {
    void api<LoginInfo>('/auth/login-info').then((i) => { setShop(i.shopName); if (i.palette) applyPalette(i.palette) }, () => {})
    return watchTrust(`https://${location.hostname}:${HTTPS_PORT}${BASE}/ping`, () => setTrusted(true))
  }, [])

  return (
    <Screen>
      <main className="mx-auto flex w-full max-w-md flex-col gap-3 px-4 py-6">
        <h1 className="font-display text-2xl font-extrabold">Set up this phone{shop ? ` for ${shop}` : ''}</h1>
        <p className="text-[14px] text-muted-foreground">Once, so the till app on this phone talks safely to the shop’s PC. Stay on the shop wifi.</p>
        <ol className="flex flex-col gap-2">
          {steps(kind, shop).map((s, i) => (
            <li key={i} className={`${CARD} flex gap-3 rounded-[var(--radius)] p-3.5 text-[14px] leading-snug`}>
              <span className="grid size-7 shrink-0 place-items-center rounded-full bg-brand-soft text-[13px] font-extrabold">{i + 1}</span>
              <span className="min-w-0">
                {s.text}
                {s.file && (
                  <a href={`${BASE}/${s.file}`} download className="mt-2 flex w-fit items-center gap-2 rounded-pill bg-brand px-4 py-2 text-sm font-extrabold text-brand-foreground">
                    <Download size={16} />Download
                  </a>
                )}
              </span>
            </li>
          ))}
        </ol>
        {trusted ? (
          <div className="flex flex-col gap-2">
            <p role="status" className="flex items-center gap-2 font-bold text-success"><CheckCircle2 size={18} />This phone trusts the shop.</p>
            <a href={app} className="rounded-[var(--radius)] bg-brand p-3.5 text-center text-base font-extrabold text-brand-foreground">Open the app</a>
            <p className="text-[12.5px] text-muted-foreground">
              {kind === 'iphone' ? 'Then tap Share → Add to Home Screen.' : 'Then tap ⋮ → Add to Home screen (or Install app).'}
            </p>
          </div>
        ) : (
          <p role="status" className="flex items-center gap-2 text-[13.5px] font-semibold text-muted-foreground">
            <Loader2 size={16} className="animate-spin" />Waiting for this phone to trust the shop…
          </p>
        )}
      </main>
    </Screen>
  )
}
