export type Mode = 'system' | 'light' | 'dark'
const KEY = 'ks-mode'

/** Per-device light/dark choice (a viewer convenience, not a shop setting). */
export function getMode(): Mode {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : 'system'
  } catch { return 'system' }
}

export function setMode(m: Mode, root: HTMLElement = document.documentElement) {
  try { m === 'system' ? localStorage.removeItem(KEY) : localStorage.setItem(KEY, m) } catch { /* private mode */ }
  if (m === 'system') delete root.dataset.theme
  else root.dataset.theme = m
}
