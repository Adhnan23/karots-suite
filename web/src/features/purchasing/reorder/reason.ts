import type { ReorderCalcView } from '@/api/types.gen'

const LEARN_WEEKS = 8 // internal/features/reorder learnDays / 7

/** "10 a day", "0.6 a day": whole numbers bare, else one decimal. */
export function perDay(n: number): string {
  const r = Math.round(n * 10) / 10
  return `${Number.isInteger(r) ? r : r.toFixed(1)} a day`
}

/** Why a product's reorder level is what it is — the line under its name on Reorder and the product form. */
export function reasonLine(c: ReorderCalcView): string {
  if (c.notSelling) return 'no sales in 90 days, none this time last year'
  const parts = [c.lastYearDaily > c.recentDaily && c.lastYearDaily === c.daily
    ? `Last year this time: ${perDay(c.lastYearDaily)} · now ${perDay(c.recentDaily)}`
    : `sells ${perDay(c.daily)}`]
  parts.push(c.rhythmFrom === 'default' ? `no deliveries recorded, planning for ${c.rhythmDays} days`
    : c.regular ? `supplier every ${c.rhythmDays} days`
      : `supplier comes irregularly, planned for ${c.rhythmDays} days`)
  if (c.safetyDays > 0) parts.push(`+ ${c.safetyDays} safety days`)
  if (c.cappedBills > 0) parts.push(`${c.cappedBills} big bill${c.cappedBills === 1 ? '' : 's'} ignored`)
  return parts.join(' · ')
}

/** "learning, 3 of 8 weeks" while an Auto level is still moving off the starting guess; "" once learned. */
export function learning(c: ReorderCalcView): string {
  if (c.learnedPct >= 100) return ''
  return `learning, ${Math.floor(c.historyDays / 7)} of ${LEARN_WEEKS} weeks`
}

/** A Fixed level far from what fully learned sales suggest (internal/features/reorder LooksOff). */
export function looksOff(level: number, c: ReorderCalcView): boolean {
  if (c.learnedPct < 100 || Math.abs(c.calcLevel - level) < 3) return false
  return c.calcLevel >= 2 * level || 2 * c.calcLevel <= level
}
