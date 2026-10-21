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
  root.dataset.palette = a.palette || 'studio'
  try {
    localStorage.setItem('ks-palette', root.dataset.palette)
  } catch {
    /* private mode */
  }
}

function readableOn(hex: string) {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  const lum = 0.2126 * r + 0.7152 * g + 0.0722 * b
  return lum > 0.179 ? '#15161a' : '#ffffff'
}
