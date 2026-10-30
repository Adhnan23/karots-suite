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
  if ('serviceWorker' in navigator) void navigator.serviceWorker.register(`${BASE}/sw.js`, { scope: '/' }).catch(() => {})
}
