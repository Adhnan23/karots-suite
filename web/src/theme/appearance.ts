import type { Appearance } from '@/api/types.gen'

/** Applies the system-locked look to <html>. Tokens in tokens.css read these.
 *  A custom brand overrides both modes, so its text colour is derived here. */
export function applyAppearance(a: Appearance, root: HTMLElement = document.documentElement) {
  if (a.brandColor) {
    root.style.setProperty('--brand', a.brandColor)
    root.style.setProperty('--brand-foreground', readableOn(a.brandColor))
  } else {
    root.style.removeProperty('--brand')
    root.style.removeProperty('--brand-foreground')
  }
  root.dataset.radius = a.radius
  root.dataset.density = a.density
}

function readableOn(hex: string) {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  const lum = 0.2126 * r + 0.7152 * g + 0.0722 * b
  return lum > 0.179 ? '#15161a' : '#ffffff'
}
