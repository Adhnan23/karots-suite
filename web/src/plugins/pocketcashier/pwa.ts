import { noteUpdate } from '@/lib/version'
import { BASE } from './api'

/** Makes the phone address an installable app that opens with no server: the
 * plugin's manifest and service worker. HTTPS only (the phone port) — the PC's
 * plain address never gets a service worker. */
export function registerPwa() {
  if (location.protocol !== 'https:') return
  let link = document.querySelector<HTMLLinkElement>('link[rel=manifest]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'manifest'
    document.head.append(link)
  }
  link.href = `${BASE}/manifest.webmanifest`
  for (const [name, content] of [['apple-mobile-web-app-capable', 'yes'], ['mobile-web-app-capable', 'yes'], ['apple-mobile-web-app-status-bar-style', 'black-translucent'], ['apple-mobile-web-app-title', 'Pocket Cashier']]) {
    if (document.querySelector(`meta[name="${name}"]`)) continue
    const m = document.createElement('meta')
    m.name = name
    m.content = content
    document.head.append(m)
  }
  if (!('serviceWorker' in navigator)) return
  // A new build lists new files, so its worker is new: look for one every minute
  // (an open app never navigates); when it takes over, reload once the phone is
  // quiet (lib/version's guards: no sale going).
  const updating = !!navigator.serviceWorker.controller
  navigator.serviceWorker.addEventListener('controllerchange', () => { if (updating) noteUpdate() })
  void navigator.serviceWorker.register(`${BASE}/sw.js`, { scope: '/' })
    .then((reg) => setInterval(() => void reg.update().catch(() => {}), 60_000))
    .catch(() => {})
}
