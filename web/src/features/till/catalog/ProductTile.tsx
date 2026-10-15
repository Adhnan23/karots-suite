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
        className={cn(TILE, 'justify-start', out && 'opacity-60')}>
        {/* A faint mark in the card's own tint fills the middle: the emoji, or the first letter. */}
        <span aria-hidden className="pointer-events-none absolute -right-1 -bottom-4 select-none font-display text-[84px] font-extrabold leading-none"
          style={{ color: `var(--chart-${(p.id % 6) + 1})`, opacity: p.emoji ? 0.3 : 0.14 }}>
          {p.emoji || p.name.trim().charAt(0).toUpperCase()}
        </span>
        <span className="line-clamp-2 pr-6 text-[14.5px] font-semibold leading-tight">{p.name}</span>
        {(note || p.badges.length > 0) && (
          <span className="flex flex-wrap gap-1">
            {note && <span className="rounded-md bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] px-1.5 text-[11px] font-bold text-warning">{note}</span>}
            {p.badges.map((b) => <span key={b} className="rounded-md bg-brand-soft px-1.5 text-[11px] font-bold">{b}</span>)}
          </span>
        )}
        <span className="relative mt-auto flex flex-col">
          <span className="whitespace-nowrap text-[17px] font-bold tabular-nums">{money(sym, p.sellingPrice)}</span>
          <span className="text-[12px] text-faint">{p.isService ? 'service' : `${qtyText(p.stockQty)} ${p.unitAbbr} left`}</span>
        </span>
      </motion.button>
      <button type="button" aria-label={`About ${p.name}`} onClick={onInfo}
        className="absolute right-1.5 top-1.5 grid h-6 w-6 place-items-center rounded-full text-faint hover:bg-muted hover:text-foreground">
        <Info size={14} />
      </button>
    </div>
  )
}
