/** Shared bits of the login / lock easter-egg games. */

/** A theme token resolved to a colour a canvas can paint with. */
export function token(name: string) {
  const el = document.createElement('span')
  el.style.color = `var(${name})`
  document.body.appendChild(el)
  const c = getComputedStyle(el).color
  el.remove()
  return c
}

/** This device's best score for a game (0 when storage is blocked). */
export function readBest(key: string) {
  try { return Number(localStorage.getItem(key)) || 0 } catch { return 0 }
}

export function saveBest(key: string, v: number) {
  try { localStorage.setItem(key, String(v)) } catch { /* private mode */ }
}
