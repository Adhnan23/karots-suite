import type { PriceLabel, Slot } from '../kit'

const blank = (s: Slot) => s.key === 'none' || !s.text.trim()

/** Which text a style with one big slot draws big: the price, wherever it is;
 * with no price, the second text; if that one is blank, the other. */
export function hero(d: PriceLabel): { hero: Slot; other: Slot; heroIsFirst: boolean } {
  let heroIsFirst = d.first.key === 'price' && d.second.key !== 'price'
  if (!heroIsFirst && blank(d.second) && !blank(d.first)) heroIsFirst = true
  return heroIsFirst ? { hero: d.first, other: d.second, heroIsFirst } : { hero: d.second, other: d.first, heroIsFirst }
}

/** "Rs 1,250.00" → small "Rs", big "1,250", small ".00"; null when it isn't a number. */
export function splitPrice(text: string): { before: string; whole: string; after: string } | null {
  const m = /^(\D*?)\s*(\d[\d,]*)(\.\d+)?\s*$/.exec(text.trim())
  return m ? { before: m[1].trim(), whole: m[2], after: m[3] ?? '' } : null
}

export const isBlank = blank
