import type { Appearance } from '@/api/types.gen'
import { THEME } from './palettes'

/** Legacy radius (before palettes owned it): kept as an inline override while a
 *  shop's palette is a built-in, so an update changes nothing on screen. */
const LEGACY_RADIUS: Record<string, [string, string]> = { sharp: ['6px', '4px'], round: ['22px', '14px'] }

/** Applies the system-locked look to <html>. Palette CSS reads data-ui + data-palette.
 *  A legacy brand overrides both modes, so its text colour is derived here. */
export function applyAppearance(a: Appearance, root: HTMLElement = document.documentElement) {
  root.dataset.ui = THEME
  const builtin = !(a.palette || '').startsWith('custom-')
  const r = builtin ? LEGACY_RADIUS[a.radius] : undefined
  if (r) {
    root.style.setProperty('--radius', r[0])
    root.style.setProperty('--radius-sm', r[1])
  } else {
    root.style.removeProperty('--radius')
    root.style.removeProperty('--radius-sm')
  }
  if (builtin && a.brandColor) {
    root.style.setProperty('--brand', a.brandColor)
    root.style.setProperty('--brand-foreground', readableOn(a.brandColor))
  } else {
    root.style.removeProperty('--brand')
    root.style.removeProperty('--brand-foreground')
  }
  applyPalette(a.palette, root)
}

/** Sets the palette on <html> and remembers it for the next pre-paint. A change of
 *  palette reloads the custom palette stylesheet (the server serves the active one). */
export function applyPalette(palette: string, root: HTMLElement = document.documentElement) {
  const next = palette || 'studio'
  const changed = root.dataset.palette !== next
  root.dataset.palette = next
  try {
    if (localStorage.getItem('ks-palette') !== next) localStorage.setItem('ks-palette', next)
  } catch {
    /* private mode */
  }
  if (changed) reloadPaletteCss()
}

/** Fetches the active custom palette again (after a save or a palette change). The
 *  new sheet replaces the old one only once it has loaded, so nothing flashes. */
export function reloadPaletteCss() {
  const old = document.getElementById('ks-palette-css') as HTMLLinkElement | null
  if (!old) return
  const next = document.createElement('link')
  next.rel = 'stylesheet'
  next.href = `/api/v1/appearance/palette.css?t=${Date.now()}`
  next.onload = next.onerror = () => {
    old.remove()
    next.id = 'ks-palette-css'
  }
  old.after(next)
}

function readableOn(hex: string) {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  const lum = 0.2126 * r + 0.7152 * g + 0.0722 * b
  return lum > 0.179 ? '#15161a' : '#ffffff'
}
