/** Kiosk lockdown (karots-pos core.js). F1–F12 never reach the browser anywhere
 * (Help, Find, Reload, fullscreen, DevTools all pull a cashier out of the app);
 * the app's own F-key handlers still run. On a counter tab F1 goes back to Sell.
 * On the till and account pages the Ctrl combos that open browser chrome,
 * Alt+←/→ (back / forward) and right-click outside text boxes are off too.
 * The system user is exempt (support needs DevTools and the console). */
export type LockAction = 'block' | 'sell' | null

const cashier = (path: string) => path.startsWith('/till') || path.startsWith('/account') || path.startsWith('/lock')

export function lockFor(path: string, e: Pick<KeyboardEvent, 'key' | 'ctrlKey' | 'metaKey' | 'shiftKey' | 'altKey'>): LockAction {
  if (/^F\d{1,2}$/.test(e.key)) return e.key === 'F1' && path.startsWith('/till/') ? 'sell' : 'block'
  if (!cashier(path)) return null
  // Not Shift variants (leaves Ctrl+Shift+R) and not the editing keys (c, v, x, a, z).
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && e.key.length === 1 && 'rfpsodg=+-0'.includes(e.key.toLowerCase())) return 'block'
  if (e.altKey && (e.key === 'ArrowLeft' || e.key === 'ArrowRight')) return 'block'
  return null
}

export function startLockdown(toSell: () => void, exempt: () => boolean) {
  document.addEventListener('keydown', (e) => {
    const a = lockFor(location.pathname, e)
    if (!a || (a === 'block' && exempt())) return
    e.preventDefault() // not stopPropagation: the till's own F-keys still fire
    if (a === 'sell') toSell()
  }, true)
  document.addEventListener('contextmenu', (e) => {
    if (exempt() || !cashier(location.pathname)) return
    const el = e.target as HTMLElement | null
    if (!(el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable))) e.preventDefault()
  }, true)
}
