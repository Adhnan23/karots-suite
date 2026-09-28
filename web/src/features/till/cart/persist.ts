import type { TillState } from './types'

/** Per-user key: an admin unlocking someone else's lock never sees their cart. */
export const cartKey = (userId: number) => `ks-till-cart-${userId}`

/** The in-progress sale survives auto-lock, reload and tab switches (sessionStorage:
 * it dies with the kiosk browser, which is the end of that shift anyway). */
export function loadCart(userId: number): TillState | null {
  try {
    const raw = sessionStorage.getItem(cartKey(userId))
    if (!raw) return null
    const s = JSON.parse(raw) as TillState
    if (!s || !Array.isArray(s.lines) || !s.draft) return null
    return { ...s, notice: null }
  } catch {
    return null
  }
}

export function saveCart(userId: number, s: TillState) {
  try {
    sessionStorage.setItem(cartKey(userId), JSON.stringify({ ...s, notice: null }))
  } catch {
    /* storage full/blocked: the till still works, it just can't recover */
  }
}
