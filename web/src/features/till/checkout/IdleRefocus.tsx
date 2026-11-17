import { useEffect } from 'react'

/** After this long with no key, tap or click, the cursor goes back to the search box. */
export const IDLE_MS = 20_000

/** Whether an idle Sell page should put the cursor back in search: not while a dialog
 * is open (it has its own fields) and not when it is already there. */
export function shouldRefocus(idleMs: number, dialogOpen: boolean, activeId: string | undefined) {
  return idleMs >= IDLE_MS && !dialogOpen && activeId !== 'till-search'
}

/** A cashier who clicked into the cart or the pay box and walked away comes back to a
 * till where the next scan lands in search. */
export function IdleRefocus() {
  useEffect(() => {
    let last = Date.now()
    const seen = () => { last = Date.now() }
    const events = ['keydown', 'pointerdown', 'wheel'] as const
    for (const e of events) window.addEventListener(e, seen, { passive: true })
    const t = window.setInterval(() => {
      const dialogOpen = !!document.querySelector('[role="dialog"],[role="alertdialog"]')
      if (shouldRefocus(Date.now() - last, dialogOpen, document.activeElement?.id)) {
        document.getElementById('till-search')?.focus()
        last = Date.now()
      }
    }, 2_000)
    return () => {
      for (const e of events) window.removeEventListener(e, seen)
      window.clearInterval(t)
    }
  }, [])
  return null
}
