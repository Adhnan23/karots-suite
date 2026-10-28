import type { Category, Kind } from './types'

export const EMOJI: Record<Category, string> = { copy: '📄', print: '🖨', laminate: '✨', bind: '📚', other: '🧾' }
export const CATEGORY: [Category, string][] = [['copy', 'Copy'], ['print', 'Print'], ['laminate', 'Laminate'], ['bind', 'Bind'], ['other', 'Other']]
export const KIND: [Kind, string][] = [['metered', 'Priced by size'], ['custom', 'Price at the till']]
export const CELLS = ['B&W 1-side', 'B&W 2-side', 'Colour 1-side', 'Colour 2-side'] as const

/** "A4 · Photo paper · Colour · 2-side", "A4 · Gloss" for a report row ('' for custom / earlier jobs). */
export function optionsText(size: string | null, material: string, color: boolean | null, double: boolean | null) {
  if (color === null && double === null) return [size, material].filter(Boolean).join(' · ')
  return [size || 'Any size', material, color ? 'Colour' : 'B&W', double ? '2-side' : '1-side'].filter(Boolean).join(' · ')
}

/** What a size's stock line is called, per kind of job: paper by the sheet, a pouch a piece, a binder a book. */
export const USES: Record<Category, { title: string; pick: string; per: string }> = {
  copy: { title: 'Paper (per sheet — 2-sided uses half)', pick: 'Pick the paper', per: 'a sheet' },
  print: { title: 'Paper (per sheet — 2-sided uses half)', pick: 'Pick the paper', per: 'a sheet' },
  laminate: { title: 'Pouch (per piece)', pick: 'Pick the pouch', per: 'a piece' },
  bind: { title: 'Binder and covers (per book)', pick: 'Pick the binder', per: 'a book' },
  other: { title: 'Paper (per sheet — 2-sided uses half)', pick: 'Pick the paper', per: 'a sheet' },
}

/** What the till's count row is called. */
export const COUNT: Record<Category, string> = { copy: 'Copies', print: 'Copies', laminate: 'Pieces', bind: 'Books', other: 'Copies' }
