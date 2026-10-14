import type { CartLine } from '@/features/till/cart/types'
import { tagOf } from './till'

/** "🔧 R-0012" across a cart line added from a repair job. */
export function LineHint({ line }: { line: CartLine }) {
  const t = tagOf(line.plugin)
  if (!t) return null
  return <span className="text-[11px] font-semibold text-muted-foreground">🔧 {t.ticket}</span>
}
