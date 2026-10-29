import { describe, expect, it } from 'vitest'
import { failing, PAIRS } from '@/features/appearance/contrast'
import { buildPalette, builtinDoc, isBuiltin, parsePalette } from '@/features/appearance/paletteCss'
import { NEW_PALETTES, PALETTES } from '.'
import { TOKENS } from './tokens'

// Every built-in added after the first five:
// it parses, sets every token in both modes, reads well, survives Duplicate, and its
// signature CSS stays safe. The first five are never edited.
const FILES = import.meta.glob<string>('/src/theme/palettes/dock/*.css', { query: '?raw', import: 'default', eager: true })

/** A token's colour as rgb(), following var() chains; null when it isn't a plain colour. */
function resolveToken(tokens: Record<string, string>, name: string, depth = 0): string | null {
  const v = tokens[name]?.trim()
  if (!v || depth > 8) return null
  const ref = /^var\((--[\w-]+)\)$/.exec(v)
  if (ref) return resolveToken(tokens, ref[1], depth + 1)
  const hex = /^#([\da-f]{3,8})$/i.exec(v)?.[1]
  if (hex) {
    const full = hex.length <= 4 ? [...hex].map((c) => c + c).join('') : hex
    const [r, g, b] = [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16))
    const a = full.length === 8 ? parseInt(full.slice(6, 8), 16) / 255 : 1
    return `rgba(${r}, ${g}, ${b}, ${a})`
  }
  return /^(rgba?|color)\(/.test(v) ? v.replace(/rgb\(([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*\/\s*([\d.]+%?)\)/, (_, r, g, b, a) => `rgba(${r}, ${g}, ${b}, ${a.endsWith('%') ? parseFloat(a) / 100 : a})`) : null
}

it('the first five stay first, in order', () => {
  expect(PALETTES.slice(0, 5).map((p) => p.id)).toEqual(['studio', 'night', 'bazaar', 'lagoon', 'catppuccin'])
  expect(PALETTES.slice(5).map((p) => p.id)).toEqual(NEW_PALETTES)
})

for (const id of NEW_PALETTES)
  describe(id, () => {
    const css = FILES[`/src/theme/palettes/dock/${id}.css`] ?? ''
    const doc = parsePalette(css)
    const modes = { light: doc.light, dark: { ...doc.light, ...doc.dark } }

    it('is registered', () => {
      const p = PALETTES.find((x) => x.id === id)
      expect(p?.name).toBeTruthy()
      expect(p?.swatch.brand).toMatch(/^#/)
      expect(isBuiltin(id)).toBe(true)
    })

    it('sets every token in light and dark', () => {
      for (const t of TOKENS) {
        expect(doc.light, `light ${t.name}`).toHaveProperty(t.name)
        expect(doc.dark, `dark ${t.name}`).toHaveProperty(t.name)
      }
    })

    for (const [mode, tokens] of Object.entries(modes))
      it(`reads well in ${mode}`, () => {
        for (const p of PAIRS) for (const t of [p.text, p.on]) expect(resolveToken(tokens, t), `${mode} ${t} is a plain colour`).not.toBeNull()
        expect(failing((t) => resolveToken(tokens, t), PAIRS)).toEqual([])
      })

    it('survives Duplicate (both modes and the signature CSS)', () => {
      const copy = parsePalette(buildPalette(builtinDoc(id), 'custom-1'))
      expect(copy.light).toEqual(doc.light)
      expect(copy.dark).toEqual(doc.dark)
      expect(copy.extra.trim()).toBe(doc.extra.trim().replace(/\[data-palette="[^"]*"\]/g, '[data-palette="custom-1"]'))
    })

    it('keeps status colours in their families (green / red / amber)', () => {
      for (const [mode, tokens] of Object.entries(modes)) {
        const h = (t: string) => hue(resolveToken(tokens, t) ?? '')
        expect(h('--success'), `${mode} success`).toSatisfy((x: number) => x >= 55 && x <= 170)
        expect(h('--danger'), `${mode} danger`).toSatisfy((x: number) => x >= 335 || x <= 15)
        expect(h('--warning'), `${mode} warning`).toSatisfy((x: number) => x >= 28 && x <= 55)
      }
    })

    it('never animates the background under a backdrop blur (a till PC would re-blur every frame)', () => {
      if (/@keyframes/.test(css)) for (const t of [doc.light, doc.dark]) expect(t['--glass']).toBe('none')
    })

    it('keeps its signature CSS safe', () => {
      expect(doc.extra).not.toMatch(/prefers-color-scheme/)
      if (/@keyframes/.test(css)) expect(css).toMatch(/prefers-reduced-motion:\s*reduce/)
      for (const d of ['--dur-fast', '--dur', '--dur-slow'])
        for (const t of [doc.light, doc.dark]) expect(parseFloat(t[d]) * (t[d]?.endsWith('ms') ? 1 : 1000), d).toBeLessThanOrEqual(350)
      expect(css.split('\n').length).toBeLessThanOrEqual(500)
    })
  })

/** Hue in degrees of an rgba() colour. */
function hue(c: string): number {
  const [r, g, b] = (c.match(/[\d.]+/g) ?? []).slice(0, 3).map((x) => Number(x) / 255)
  const max = Math.max(r, g, b)
  const d = max - Math.min(r, g, b)
  if (!d) return 0
  const h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4
  return (h * 60 + 360) % 360
}

it('Aurora puts its gradient on the kit\'s main buttons too, not only hand-styled brand fills', () => {
  const extra = parsePalette(FILES['/src/theme/palettes/dock/aurora.css']).extra
  expect(extra).toMatch(/\.bg-brand/)
  expect(extra).toMatch(/\[data-slot="button"\]\[data-variant="default"\]/)
})

it('a reduced-transparency fallback never gives a system-light device the dark colours', () => {
  for (const [file, css] of Object.entries(FILES)) {
    // A `:not([data-theme="light"])` rule applies in system mode, so it must sit under prefers-color-scheme: dark.
    for (const block of css.split(/(?=@media)/)) {
      const prelude = block.slice(0, block.indexOf('{'))
      if (/prefers-reduced-transparency/.test(prelude) && !/prefers-color-scheme:\s*dark/.test(prelude))
        expect(block.slice(0, block.lastIndexOf('}')), file).not.toMatch(/:not\(\[data-theme="light"\]\)\s*[,{][^}]*--card:\s*#/)
    }
  }
})

it('Neumorph leaves an invalid field its own red halo', () => {
  const extra = parsePalette(FILES['/src/theme/palettes/dock/neumorph.css']).extra
  expect(extra).toMatch(/input[^{]*:not\(\[aria-invalid="true"\]\)/)
})

it('Aurora dark: faint text still reads where the glow sits behind a card', () => {
  const d = parsePalette(FILES['/src/theme/palettes/dock/aurora.css'])
  const dark = { ...d.light, ...d.dark }
  // The brightest spot: the teal glow (rgb(0 229 192 / .2)) over the dark page, under the see-through card.
  const bg = resolveToken(dark, '--background')!.match(/[\d.]+/g)!.map(Number)
  const glow = [0, 229, 192].map((c, i) => Math.round(c * 0.2 + bg[i] * 0.8))
  const page = `rgba(${glow.join(', ')}, 1)`
  const onGlow = failing((k) => (k === '--background' ? page : resolveToken(dark, k)), [{ text: '--faint', on: '--card', min: 3 }])
  expect(onGlow).toEqual([])
})

describe('resolveToken', () => {
  it('follows var() and reads hex / rgb', () => {
    const t = { '--a': '#fff', '--b': 'var(--a)', '--c': 'rgb(10 20 30 / .5)', '--d': 'linear-gradient(red, blue)' }
    expect(resolveToken(t, '--b')).toBe('rgba(255, 255, 255, 1)')
    expect(resolveToken(t, '--c')).toBe('rgba(10, 20, 30, .5)')
    expect(resolveToken(t, '--d')).toBeNull()
  })
})

// The cheque colour is in every built-in, the first five too: readable on
// cards, its text readable on it, and a hue of its own (not brand / status).
describe('cheque colour', () => {
  const pairs = PAIRS.filter((p) => p.text.startsWith('--cheque'))
  for (const p of PALETTES) {
    const doc = parsePalette(FILES[`/src/theme/palettes/dock/${p.id}.css`] ?? '')
    for (const [mode, tokens] of Object.entries({ light: doc.light, dark: { ...doc.light, ...doc.dark } }))
      it(`${p.id} ${mode}`, () => {
        expect(pairs).toHaveLength(2)
        expect(failing((t) => resolveToken(tokens, t), pairs)).toEqual([])
        const h = hue(resolveToken(tokens, '--cheque') ?? '')
        for (const other of ['--success', '--danger', '--warning']) {
          const d = Math.abs(h - hue(resolveToken(tokens, other) ?? ''))
          expect(Math.min(d, 360 - d), `${mode} cheque vs ${other}`).toBeGreaterThanOrEqual(30)
        }
      })
  }
})
