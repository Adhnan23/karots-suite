/** Colour values as the editor's picker sees them: a hex plus an opacity, written
 *  back as hex (solid) or `rgb(r g b / a)` (see-through), the palettes' own forms. */
export type Picked = { hex: string; alpha: number }

const hex2 = (n: number) => Math.round(Math.min(255, Math.max(0, n))).toString(16).padStart(2, '0').toUpperCase()

export function parseCssColour(v: string): Picked | null {
  const s = v.trim()
  let m = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(s)
  if (m) {
    const h = m[1].length === 3 ? [...m[1]].map((c) => c + c).join('') : m[1]
    return { hex: `#${h.toUpperCase()}`, alpha: 1 }
  }
  m = /^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)(?:\s*[,/]\s*([\d.]+))?\s*\)$/.exec(s)
  if (m) return { hex: `#${hex2(+m[1])}${hex2(+m[2])}${hex2(+m[3])}`, alpha: m[4] === undefined ? 1 : +m[4] }
  m = /^color\(srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+))?\s*\)$/.exec(s)
  if (m) return { hex: `#${hex2(+m[1] * 255)}${hex2(+m[2] * 255)}${hex2(+m[3] * 255)}`, alpha: m[4] === undefined ? 1 : +m[4] }
  return null
}

export function formatColour(hex: string, alpha: number): string {
  if (alpha >= 1) return hex.toUpperCase()
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16))
  return `rgb(${r} ${g} ${b} / ${String(Math.round(alpha * 100) / 100).replace(/^0/, '')})`
}

/** What the browser draws for any colour expression (var(), color-mix()…). */
export function drawnColour(value: string): Picked | null {
  const probe = document.createElement('span')
  probe.style.display = 'none'
  probe.style.color = value
  document.body.append(probe)
  const c = getComputedStyle(probe).color
  probe.remove()
  return parseCssColour(c)
}
