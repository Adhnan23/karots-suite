import { Info } from 'lucide-react'
import type { TillProduct } from '@/api/types.gen'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { LowDot, Marks } from './ProductTile'

/** A product as one row (List cards, Appearance → Sell page layout): name and local
 * name, its marks, stock, price. ↑ ↓ in search highlight it; ⓘ stays. */
export function ProductRow({ p, sym, note, onAdd, onInfo, active = false }: {
  p: TillProduct
  sym: string
  note: string | null
  onAdd: () => void
  onInfo: () => void
  active?: boolean
}) {
  const out = !p.isService && !p.stockFrozen && num(p.stockQty) <= 0
  return (
    <div className={cn('relative flex items-center border-b border-border last:border-b-0', active && 'bg-brand-soft')}
      ref={(el) => { if (active) el?.scrollIntoView({ block: 'nearest' }) }}>
      <button type="button" onClick={onAdd} aria-current={active || undefined}
        className={cn('flex min-w-0 flex-1 items-center gap-3 py-2 pr-2 pl-3 text-left hover:bg-muted', out && 'opacity-60')}>
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate text-till-body font-semibold">
            {p.name}{p.nameLocal && <span className="ml-2 font-medium text-muted-foreground">{p.nameLocal}</span>}
          </span>
          <Marks p={p} note={note} small />
        </span>
        <span className="shrink-0 whitespace-nowrap text-right text-till-sm text-faint tabular-nums">
          <LowDot p={p} />{p.isService ? 'service' : p.stockFrozen ? 'frozen' : `${qtyText(p.stockQty)} ${p.unitAbbr}`}
        </span>
        <span className="w-24 shrink-0 whitespace-nowrap text-right font-display text-till-lead font-bold tabular-nums">{money(sym, p.sellingPrice)}</span>
      </button>
      <button type="button" tabIndex={-1} aria-label={`About ${p.name}`} onClick={onInfo}
        className="mr-1.5 grid h-6 w-6 shrink-0 place-items-center rounded-full text-faint hover:bg-muted hover:text-foreground">
        <Info size={14} />
      </button>
    </div>
  )
}
