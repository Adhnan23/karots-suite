import { money } from '@/lib/money'
import type { Combo } from './types'

/** "−10%", "Rs 50 off", "Combo Rs 500". */
export function dealLabel(c: Pick<Combo, 'discountType' | 'discountValue'>, sym: string) {
  if (c.discountType === 'percent') return `${Number(c.discountValue)}% off`
  if (c.discountType === 'fixed') return `${money(sym, c.discountValue)} off`
  return `Combo ${money(sym, c.discountValue)}`
}

const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })
/** "today", "2 days ago". */
export function ago(iso: string, now = Date.now()) {
  const days = Math.round((new Date(iso).setHours(0, 0, 0, 0) - new Date(now).setHours(0, 0, 0, 0)) / 86_400_000)
  return rtf.format(days, 'day')
}

/** "Sold 23× · Rs 4,600 · last 2 days ago" (empty until the first sale). */
export function soldLabel(c: Pick<Combo, 'soldSets' | 'soldNet' | 'lastSold'>, sym: string) {
  if (!c.lastSold || !(Number(c.soldSets) > 0)) return ''
  return `Sold ${Number(c.soldSets)}× · ${money(sym, c.soldNet)} · last sold ${ago(c.lastSold)}`
}
