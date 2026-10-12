import type { Category, Kind } from './types'

export const EMOJI: Record<Category, string> = { copy: '📄', print: '🖨', laminate: '✨', bind: '📚', other: '🧾' }
export const CATEGORY: [Category, string][] = [['copy', 'Copy'], ['print', 'Print'], ['laminate', 'Laminate'], ['bind', 'Bind'], ['other', 'Other']]
export const KIND: [Kind, string][] = [['metered', 'Priced by size'], ['custom', 'Price at the till']]
export const CELLS = ['B&W 1-side', 'B&W 2-side', 'Colour 1-side', 'Colour 2-side'] as const

/** "A4 · Colour · 2-side" for a report row ('' for custom / earlier jobs). */
export function optionsText(size: string | null, color: boolean | null, double: boolean | null) {
  if (color === null && double === null) return size ?? ''
  return [size || 'Any size', color ? 'Colour' : 'B&W', double ? '2-side' : '1-side'].join(' · ')
}
