// A palette's light colours, read from its own file, so a shop card shows the
// look the customer will get.
const files = import.meta.glob<string>('../src/theme/palettes/dock/*.css', { query: '?raw', import: 'default', eager: true })

export type Swatch = { background: string; card: string; foreground: string; brand: string; brand2: string; hero: string }

export function swatch(palette: string): Swatch | null {
  const css = Object.entries(files).find(([p]) => p.endsWith(`/${palette}.css`))?.[1]
  if (!css) return null
  const light = css.slice(0, css.indexOf('}'))
  const v = (name: string) => light.match(new RegExp(`--${name}:\\s*([^;]+);`))?.[1].trim() ?? ''
  return { background: v('background'), card: v('card'), foreground: v('foreground'), brand: v('brand'), brand2: v('brand-2'), hero: v('hero') }
}
