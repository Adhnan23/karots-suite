import { Info } from 'lucide-react'
import { motion } from 'motion/react'
import type { Lot, TillProduct } from '@/api/types.gen'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lotExpired, lotsFor } from '../cart/lines'
import { TILE } from '../ui'

/** A small cue that this product will ask "which price?". */
export function lotNote(opts: Record<string, Lot[]> | undefined, id: number) {
  const lots = lotsFor(opts, id)
  if (!lots.length) return null
  return lotExpired(lots[0]) ? 'Expired lot first' : `${new Set(lots.map((l) => l.price)).size} prices`
}

export function ProductTile({ p, sym, note, onAdd, onInfo }: {
  p: TillProduct
  sym: string
  note: string | null
  onAdd: () => void
  onInfo: () => void
}) {
  const out = !p.isService && num(p.stockQty) <= 0
  return (
    <div className="relative">
      <motion.button type="button" onClick={onAdd} whileHover={{ y: -2 }} whileTap={{ scale: 0.97 }} transition={{ duration: 0.12 }}
        className={cn(TILE, 'pr-8', out && 'opacity-60')}>
        <span className="text-[13.5px] font-semibold leading-tight">
          {p.emoji && <span className="mr-1">{p.emoji}</span>}
          {p.name}
        </span>
        {(note || p.badges.length > 0) && (
          <span className="flex flex-wrap gap-1">
            {note && <span className="rounded-md bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] px-1.5 text-[11px] font-bold text-warning">{note}</span>}
            {p.badges.map((b) => <span key={b} className="rounded-md bg-brand-soft px-1.5 text-[11px] font-bold">{b}</span>)}
          </span>
        )}
        <span className="flex flex-col">
          <span className="whitespace-nowrap text-[15px] font-bold tabular-nums">{money(sym, p.sellingPrice)}</span>
          <span className="text-[11.5px] text-faint">{p.isService ? 'service' : `${qtyText(p.stockQty)} ${p.unitAbbr} left`}</span>
        </span>
      </motion.button>
      <button type="button" aria-label={`About ${p.name}`} onClick={onInfo}
        className="absolute right-1.5 top-1.5 grid h-6 w-6 place-items-center rounded-full text-faint hover:bg-muted hover:text-foreground">
        <Info size={14} />
      </button>
    </div>
  )
}
