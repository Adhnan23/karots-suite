import { dec, num } from '@/lib/money'

export type Prices = { cost: string; markup: string; selling: string }

/** Markup as "1.25" (selling ÷ cost), '' when there's no cost to divide by. */
export const markupOf = (cost: string, selling: string) => (num(cost) > 0 && num(selling) > 0 ? String(Math.round((num(selling) / num(cost)) * 100) / 100) : '')

/** Two-way cost ⇄ markup ⇄ selling: type any one, the others follow. Cost or
 * markup set the selling price; selling sets the markup. whole = round a derived
 * price to the rupee, else to the cent. */
export function editPrice(p: Prices, field: keyof Prices, value: string, whole = false): Prices {
  const next = { ...p, [field]: value }
  if (field === 'selling') return { ...next, markup: markupOf(next.cost, value) }
  const fmt = (n: number) => (whole ? String(Math.round(n)) : dec(n))
  if (num(next.cost) > 0 && num(next.markup) > 0) return { ...next, selling: fmt(num(next.cost) * num(next.markup)) }
  return field === 'cost' ? { ...next, markup: markupOf(value, next.selling) } : next
}

/** Margin on the selling price, "%" text ('' when unknown). */
export const marginText = (cost: string, selling: string) =>
  num(selling) > 0 && num(cost) > 0 ? `${Math.round(((num(selling) - num(cost)) / num(selling)) * 1000) / 10}% margin` : ''
