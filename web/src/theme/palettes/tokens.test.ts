import { describe, expect, it } from 'vitest'
import { TOKENS } from './tokens'

const files = import.meta.glob<string>('./dock/*.css', { query: '?raw', import: 'default', eager: true })

/** The declarations of the first top-level block whose selector matches. */
function block(css: string, selector: RegExp) {
  const m = selector.exec(css)
  if (!m) return ''
  const start = css.indexOf('{', m.index) + 1
  return css.slice(start, css.indexOf('}', start))
}

describe('built-in palettes', () => {
  it('there are palette files', () => expect(Object.keys(files).length).toBeGreaterThanOrEqual(2))
  for (const [path, css] of Object.entries(files)) {
    const id = path.replace(/^.*\/(.+)\.css$/, '$1')
    it(`${id} defines every token in light and dark`, () => {
      const light = block(css, new RegExp(`^:root\\[data-ui="dock"\\]\\[data-palette="${id}"\\] \\{`, 'm'))
      const dark = block(css, new RegExp(`^:root\\[data-ui="dock"\\]\\[data-palette="${id}"\\]\\[data-theme="dark"\\] \\{`, 'm'))
      for (const t of TOKENS) {
        expect(light, `${id} light ${t.name}`).toMatch(new RegExp(`${t.name}:`))
        expect(dark, `${id} dark ${t.name}`).toMatch(new RegExp(`${t.name}:`))
      }
    })
    it(`${id} scopes every selector to data-ui="dock"`, () => {
      for (const sel of css.match(/^\s*:root[^{,]*/gm) ?? []) expect(sel).toContain('[data-ui="dock"]')
    })
  }
})
