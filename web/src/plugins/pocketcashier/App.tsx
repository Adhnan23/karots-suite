import { useCallback, useEffect, useState } from 'react'
import { api } from '@/api/client'
import type { LoginInfo, TillCopy } from '@/api/types.gen'
import { applyPalette } from '@/theme/appearance'
import { registerPwa } from './pwa'
import { Pair } from './screens/Pair'
import { Phone } from './screens/Phone'
import { Setup } from './screens/Setup'
import { SignIn } from './screens/SignIn'
import { loadWho, saveWho, type Who } from './session'
import { loadCopy } from './store/copy'
import { loadPhone, type PhoneId } from './store/phone'
import { startSync, type SyncState } from './sync'

/** The surface at /x/pocketcashier: `setup` is the plain-HTTP page that gets a
 * phone to trust the shop; anything else is the phone app. */
export default function App({ path }: { path: string }) {
  return path.replace(/\/$/, '') === 'setup' ? <Setup /> : <PhoneApp />
}

/** Pair once, then sign in and sell — live, or from the copy when the PC is down. */
function PhoneApp() {
  const [phone, setPhone] = useState<PhoneId | null | undefined>(undefined)
  const [who, setWho] = useState<Who | null>(loadWho)
  const [sync, setSync] = useState<SyncState>({ live: false, copyAt: null, waiting: 0 })
  const [copy, setCopy] = useState<TillCopy | null>(null)
  const [notice, setNotice] = useState('')

  useEffect(() => {
    registerPwa()
    void loadPhone().then(setPhone, () => setPhone(null))
    // The shop's palette (cached for offline starts by applyPalette).
    void api<LoginInfo>('/auth/login-info').then((i) => i.palette && applyPalette(i.palette), () => {})
  }, [])
  useEffect(() => {
    if (!phone) return
    return startSync(setSync, () => loadWho()?.id ?? null)
  }, [phone])
  useEffect(() => { void loadCopy().then(setCopy) }, [sync.copyAt])

  const signIn = useCallback((w: Who | null) => { saveWho(w); setWho(w); if (w) setNotice('') }, [])
  if (phone === undefined) return null
  if (!phone) return <Pair onPaired={setPhone} />
  if (!who) return <SignIn copy={copy} sync={sync} phone={phone} notice={notice} onSignedIn={signIn} />
  return <Phone who={who} copy={copy} sync={sync} phone={phone} onSignOut={(why) => { setNotice(why ?? ''); signIn(null) }} />
}
