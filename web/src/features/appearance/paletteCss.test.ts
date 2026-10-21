import { describe, expect, it } from 'vitest'
import { buildPalette, builtinDoc, paletteId, parsePalette, type PaletteDoc } from './paletteCss'

const doc: PaletteDoc = {
  theme: 'dock',
  name: 'Sea glass',
  base: 'lagoon',
  light: { '--background': '#E6F4F4', '--brand': '#087F88', '--x-unknown': '1px' },
  dark: { '--background': '#071A1F', '--brand': '#4FD8DE' },
  extra: '.till-total { letter-spacing: .02em; }\n\n@media print {\n  .x { color: red; }\n}',
}

describe('palette CSS', () => {
  it('build → parse round-trips', () => {
    expect(parsePalette(buildPalette(doc, 'custom-4'))).toEqual(doc)
  })
  it('names only its own id', () => {
    const css = buildPalette(doc, 'custom-4')
    expect(css.match(/data-palette="custom-4"/g)).toHaveLength(3)
    expect(css.match(/data-palette="/g)).toHaveLength(3)
    expect(css).toContain('/* karots-palette theme=dock name=Sea glass base=lagoon */')
  })
  it('keeps token order from TOKENS, unknown tokens after', () => {
    const css = buildPalette(doc, 'custom-4')
    expect(css.indexOf('--background')).toBeLessThan(css.indexOf('--brand'))
    expect(css.indexOf('--brand:')).toBeLessThan(css.indexOf('--x-unknown'))
  })
  it('reads the theme from the header, so another theme can be refused', () => {
    const other = buildPalette(doc, 'custom-1').replace('theme=dock', 'theme=other')
    expect(parsePalette(other).theme).toBe('other')
  })
  it('reads a built-in file', () => {
    const night = builtinDoc('night')
    expect(night.dark['--brand']).toBe('#8B7BFF')
    expect(night.light['--brand']).toBe('#6E5BFF')
    expect(night).toMatchObject({ theme: 'dock', name: 'Night', base: 'night' })
    // its reduced-transparency block is extra CSS, not a token block
    expect(night.extra).toContain('prefers-reduced-transparency')
  })
  it('paletteId', () => expect(paletteId(7)).toBe('custom-7'))
})
