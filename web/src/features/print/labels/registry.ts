import { slots } from '@/plugins/slots'
import type { LabelKind, LabelPaper, LabelStyle } from './kit'
import { PRICE_KIND } from './price'

/** Every label kind in this build: core's price label, then the plugins'. */
export const labelKinds = (): LabelKind[] => [PRICE_KIND as LabelKind, ...slots.labelTemplates]

export const kindOf = (kind: string): LabelKind | undefined => labelKinds().find((k) => k.kind === kind)

/** The style that draws `kind` in style `key` on this paper with this data:
 * unknown → classic; too narrow → the style's fallback; too small for its codes
 * → classic. `fellBack` says the chosen style couldn't draw here. */
export function styleFor(kind: LabelKind, key: string, paper: LabelPaper, d: unknown): { style: LabelStyle<unknown>; fellBack: boolean } {
  const classic = kind.styles[0]
  const byKey = (k: string | undefined) => kind.styles.find((s) => s.key === k)
  let style = byKey(key) ?? classic
  if (style.minWidthMm && paper.wMm < style.minWidthMm) style = byKey(style.fallback) ?? classic
  if (style.fits && !style.fits(d, paper)) return { style: classic, fellBack: true }
  return { style, fellBack: false }
}
