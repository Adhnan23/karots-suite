export type Touch = 'auto' | 'on' | 'off'
const KEY = 'ks-touch'

/** Per-device touch keys (the till's note tray and keypad): auto = only on a touch
 * screen tall enough for them; on / off = always / never. A device convenience,
 * not a shop setting — like the colour mode. */
export function getTouch(): Touch {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'on' || v === 'off' ? v : 'auto'
  } catch { return 'auto' }
}

export function setTouch(t: Touch, root: HTMLElement = document.documentElement) {
  try { t === 'auto' ? localStorage.removeItem(KEY) : localStorage.setItem(KEY, t) } catch { /* private mode */ }
  if (t === 'auto') delete root.dataset.touch
  else root.dataset.touch = t
}
