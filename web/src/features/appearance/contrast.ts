/** Readability check for the palette editor: WCAG contrast of the token pairs the
 *  screens actually put together, measured on the colours the browser resolved
 *  (so var(), color-mix() and see-through glass all count). */
export type RGBA = [number, number, number, number]
export type Pair = { text: string; on: string; min: number; what?: string }

/** The pairs every screen relies on. 4.5 = body text; 3 = large / bold figures and icons. */
export const PAIRS: Pair[] = [
  { text: '--foreground', on: '--background', min: 4.5, what: 'Text on the page' },
  { text: '--foreground', on: '--card', min: 4.5, what: 'Text on cards' },
  { text: '--muted-foreground', on: '--card', min: 4.5, what: 'Muted text on cards' },
  { text: '--faint', on: '--card', min: 3, what: 'Faint text on cards' },
  { text: '--popover-foreground', on: '--popover', min: 4.5, what: 'Text in menus and dialogs' },
  { text: '--accent-foreground', on: '--accent', min: 4.5, what: 'Hovered menu items and plain buttons' },
  { text: '--brand-foreground', on: '--brand', min: 4.5, what: 'Text on brand buttons' },
  { text: '--brand-2-foreground', on: '--brand-2', min: 3, what: 'Bold text on the second brand' },
  { text: '--selected-foreground', on: '--selected', min: 4.5, what: 'Selected tabs and segments' },
  { text: '--danger-foreground', on: '--danger', min: 4.5, what: 'Text on delete buttons' },
  { text: '--success', on: '--card', min: 3, what: 'Money in on cards' },
  { text: '--danger', on: '--card', min: 3, what: 'Money out on cards' },
  { text: '--warning', on: '--card', min: 3, what: 'Warnings on cards' },
  { text: '--hero-foreground', on: '--hero', min: 4.5, what: 'Text on the hero' },
  { text: '--hero-muted', on: '--hero', min: 3, what: 'Muted text on the hero' },
]

export function parseColour(s: string): RGBA | null {
  let m = /^rgba?\(\s*([\d.]+),?\s+([\d.]+),?\s+([\d.]+)(?:\s*[,/]\s*([\d.]+))?\s*\)$/.exec(s.trim())
  if (m) return [+m[1], +m[2], +m[3], m[4] === undefined ? 1 : +m[4]]
  m = /^color\(srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+))?\s*\)$/.exec(s.trim())
  if (m) return [+m[1] * 255, +m[2] * 255, +m[3] * 255, m[4] === undefined ? 1 : +m[4]]
  return null
}

/** top painted over bottom (bottom must be opaque). */
function over(top: RGBA, bottom: RGBA): RGBA {
  const a = top[3]
  return [0, 1, 2].map((i) => top[i] * a + bottom[i] * (1 - a)).concat(1) as RGBA
}

function luminance([r, g, b]: RGBA) {
  const f = (c: number) => ((c /= 255) <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b)
}

export function ratio(a: RGBA, b: RGBA) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

/** Pairs below their minimum. colour(token) returns the browser's resolved value. */
export function failing(colour: (token: string) => string | null, pairs: Pair[] = PAIRS) {
  const page = parseColour(colour('--background') ?? '') ?? [255, 255, 255, 1]
  const solidPage = over(page, [255, 255, 255, 1])
  const out: (Pair & { ratio: number })[] = []
  for (const p of pairs) {
    const on = parseColour(colour(p.on) ?? '')
    const text = parseColour(colour(p.text) ?? '')
    if (!on || !text) continue
    const bg = over(on, solidPage)
    const r = ratio(over(text, bg), bg)
    if (r < p.min) out.push({ ...p, ratio: Math.floor(r * 100) / 100 }) // never rounds a fail up to the minimum
  }
  return out
}

/** Resolves tokens through the live page (the draft palette and preview mode apply). */
export function resolver(): (token: string) => string | null {
  const probe = document.createElement('span')
  probe.style.display = 'none'
  document.body.append(probe)
  const cache = new Map<string, string>()
  const read = (t: string) => {
    if (!cache.has(t)) {
      probe.style.color = `var(${t})`
      cache.set(t, getComputedStyle(probe).color)
    }
    return cache.get(t) ?? null
  }
  queueMicrotask(() => probe.remove())
  return read
}
