import { describe, expect, it } from 'vitest'
import { failing, PAIRS } from '@/features/appearance/contrast'
import { buildPalette, builtinDoc, isBuiltin, parsePalette } from '@/features/appearance/paletteCss'
import { NEW_PALETTES, PALETTES } from '.'
import { TOKENS } from './tokens'

// Every built-in added after the first five (docs/specs/2026-10-27-more-palettes-design.md):
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

    it('keeps its signature CSS safe', () => {
      expect(doc.extra).not.toMatch(/prefers-color-scheme/)
      if (/@keyframes/.test(css)) expect(css).toMatch(/prefers-reduced-motion:\s*reduce/)
      for (const d of ['--dur-fast', '--dur', '--dur-slow'])
        for (const t of [doc.light, doc.dark]) expect(parseFloat(t[d]) * (t[d]?.endsWith('ms') ? 1 : 1000), d).toBeLessThanOrEqual(350)
      expect(css.split('\n').length).toBeLessThanOrEqual(500)
    })
  })

describe('resolveToken', () => {
  it('follows var() and reads hex / rgb', () => {
    const t = { '--a': '#fff', '--b': 'var(--a)', '--c': 'rgb(10 20 30 / .5)', '--d': 'linear-gradient(red, blue)' }
    expect(resolveToken(t, '--b')).toBe('rgba(255, 255, 255, 1)')
    expect(resolveToken(t, '--c')).toBe('rgba(10, 20, 30, .5)')
    expect(resolveToken(t, '--d')).toBeNull()
  })
})
