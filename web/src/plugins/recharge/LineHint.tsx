import type { CartLine } from '@/features/till/cart/types'
import { tagOf } from './till'

/** "📶 Dialog · Shop phone" across a recharge line: where its float / money is. */
export function LineHint({ line }: { line: CartLine }) {
  const t = tagOf(line.plugin)
  if (!t?.label) return null
  return <span className="text-[11px] font-semibold text-muted-foreground">{t.bank ? '🏦' : '📶'} {t.label}</span>
}
