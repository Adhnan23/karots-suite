import type { RecipeComponent, RecipeCost } from '@/api/types.gen'
import { num, r2 } from '@/lib/money'

/** What one unit sold uses of an ingredient — recipes.Component.Consumed(1):
 * a yield divides, a whole-unit ingredient rounds up. */
export function usedPerSale(c: Pick<RecipeComponent, 'amount' | 'mode' | 'whole'>): number {
  const a = num(c.amount)
  if (!(a > 0)) return 0
  const per = c.mode === 'yield' ? 1 / a : a
  return c.whole ? Math.ceil(per) : per
}

/** The recipe's cost per unit sold: stock (becomes COGS) and other (estimate only). */
export function recipeCost(cs: RecipeComponent[], costs: RecipeCost[]) {
  const stock = r2(cs.reduce((s, c) => s + usedPerSale(c) * num(c.costPrice), 0))
  const other = r2(costs.reduce((s, l) => s + num(l.amount), 0))
  return { stock, other, total: r2(stock + other) }
}
