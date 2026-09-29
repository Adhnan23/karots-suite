import { dec, num } from '@/lib/money'

export type Prices = { cost: string; markup: string; selling: string }

/** Markup as "1.25" (selling ÷ cost), '' when there's no cost to divide by. */
export const markupOf = (cost: string, selling: string) => (num(cost) > 0 && num(selling) > 0 ? String(Math.round((num(selling) / num(cost)) * 100) / 100) : '')

/** Two-way cost ⇄ markup ⇄ selling (karots-pos intake): type any one, the
 * others follow. Cost or markup set the selling price; selling sets the markup. */
export function editPrice(p: Prices, field: keyof Prices, value: string): Prices {
  const next = { ...p, [field]: value }
  if (field === 'selling') return { ...next, markup: markupOf(next.cost, value) }
  if (num(next.cost) > 0 && num(next.markup) > 0) return { ...next, selling: dec(num(next.cost) * num(next.markup)) }
  return field === 'cost' ? { ...next, markup: markupOf(value, next.selling) } : next
}

/** Margin on the selling price, "%" text ('' when unknown). */
export const marginText = (cost: string, selling: string) =>
  num(selling) > 0 && num(cost) > 0 ? `${Math.round(((num(selling) - num(cost)) / num(selling)) * 1000) / 10}% margin` : ''
