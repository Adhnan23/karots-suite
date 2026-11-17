import { Info } from 'lucide-react'
import { motion } from 'motion/react'
import { tx } from '@/theme/motion'
import type { Lot, TillProduct } from '@/api/types.gen'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lotExpired, lotsFor } from '../cart/lines'
import { TILE } from '../ui'

/** A small cue that this product will ask "which price?". */
export function lotNote(opts: Record<string, Lot[]> | undefined, id: number) {
  const lots = lotsFor(opts, id)
  if (!lots.length) return null
  return lots[0].recalled ? 'Recalled lot first' : lotExpired(lots[0]) ? 'Expired lot first' : `${new Set(lots.map((l) => l.price)).size} prices`
}

/** Card size (Appearance → Sell page layout): Large (today), Medium, Small. List is ProductRow. */
export type CardSize = 'L' | 'M' | 'S'

const out = (p: TillProduct) => !p.isService && !p.stockFrozen && num(p.stockQty) <= 0
export const stockText = (p: TillProduct) => (p.isService ? 'service' : p.stockFrozen ? 'stock frozen' : `${qtyText(p.stockQty)} ${p.unitAbbr} left`)

/** Frozen, Low stock, a lot note and plugin badges: kept at every card size (a cashier
 * must still see them); Small and List show low stock as a dot beside the stock. */
export function Marks({ p, note, small = false }: { p: TillProduct; note: string | null; small?: boolean }) {
  const low = p.lowStock && !out(p) && !p.stockFrozen && !small
  if (!(note || p.stockFrozen || low || p.badges.length > 0)) return null
  const chip = small ? 'rounded-md px-1.5 text-till-xs font-bold' : 'rounded-md px-2 py-0.5 text-till-sm font-bold'
  return (
    <span className="flex flex-wrap gap-1">
      {p.stockFrozen && <span className={cn(chip, 'bg-brand-soft')}>Frozen</span>}
      {low && <span className={cn(chip, 'bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] text-danger')}>Low stock</span>}
      {note && <span className={cn(chip, 'bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] text-warning')}>{note}</span>}
      {p.badges.map((b) => <span key={b} className={cn(chip, 'bg-brand-soft')}>{b}</span>)}
    </span>
  )
}

/** A red dot before the stock when it's low (Small cards and List rows). */
export const LowDot = ({ p }: { p: TillProduct }) =>
  p.lowStock && !out(p) && !p.stockFrozen ? <span aria-label="Low stock" className="mr-1 inline-block h-2 w-2 rounded-full bg-danger" /> : null

export function ProductTile({ p, sym, note, onAdd, onInfo, active = false, size = 'L' }: {
  p: TillProduct
  sym: string
  note: string | null
  onAdd: () => void
  onInfo: () => void
  /** Picked with ↑ ↓ from the search box: Enter adds it. */
  active?: boolean
  size?: CardSize
}) {
  // Counter-safe facts only (never cost, margin or the wholesale price — the customer
  // can see this screen): what a cashier gets asked about. Large cards only.
  const facts = size !== 'L' ? [] : [
    ...p.packs.map((k) => `${k.name} ×${qtyText(k.size)} · ${money(sym, num(k.sellingPrice) > 0 ? k.sellingPrice : num(k.size) * num(p.sellingPrice))}`),
    p.warrantyMonths > 0 && `${p.warrantyMonths} mo warranty`,
    p.trackSerial && 'Serial no.',
    p.unitAllowDecimal && `Sold by ${p.unitAbbr}`,
    p.barcode.length > 4 && `#…${p.barcode.slice(-4)}`,
  ].filter(Boolean)
  return (
    <div className="relative" ref={(el) => { if (active) el?.scrollIntoView({ block: 'nearest' }) }}>
      <motion.button type="button" onClick={onAdd} whileHover={{ y: -2 }} whileTap={{ scale: 0.97 }} transition={tx('fast')}
        aria-current={active || undefined}
        className={cn(TILE, 'justify-start', size === 'M' && 'min-h-0 gap-1.5 p-2.5', size === 'S' && 'min-h-0 gap-1 px-2.5 py-2', out(p) && 'opacity-60', active && 'ring-2 ring-ring')}>
        {/* A faint mark in the card's own tint fills the middle of a Large card: the emoji, or the first letter. */}
        {size === 'L' && (
          <span aria-hidden className="pointer-events-none absolute -right-1 -bottom-4 select-none font-display text-[96px] font-extrabold leading-none"
            style={{ color: `var(--chart-${(p.id % 6) + 1})`, opacity: p.emoji ? 0.3 : 0.14 }}>
            {p.emoji || p.name.trim().charAt(0).toUpperCase()}
          </span>
        )}
        <span className={cn('line-clamp-2 pr-6 font-semibold leading-tight', size === 'S' ? 'text-till-body' : 'text-till-lead')}>{p.name}</span>
        {p.nameLocal && size !== 'S' && <span className="relative -mt-1 line-clamp-1 text-till-body font-medium text-muted-foreground">{p.nameLocal}</span>}
        {p.line && size === 'L' && <span className="relative -mt-1 line-clamp-2 pr-6 text-till-sm leading-snug text-muted-foreground">{p.line}</span>}
        <Marks p={p} note={note} small={size === 'S'} />
        {facts.length > 0 && <span className="relative line-clamp-2 text-till-sm font-medium text-muted-foreground">{facts.join(' · ')}</span>}
        {size === 'L' ? (
          <span className="relative mt-auto flex flex-col">
            <span className="whitespace-nowrap font-display text-till-price font-bold leading-tight tabular-nums">{money(sym, p.sellingPrice)}</span>
            <span className="text-till-sm text-faint">{stockText(p)}</span>
          </span>
        ) : (
          <span className="relative mt-auto flex items-baseline justify-between gap-2">
            <span className={cn('whitespace-nowrap font-display font-bold leading-tight tabular-nums', size === 'S' ? 'text-till-lead' : 'text-till-xl')}>{money(sym, p.sellingPrice)}</span>
            <span className="truncate text-till-sm text-faint">{size === 'S' && <LowDot p={p} />}{p.isService ? '' : p.stockFrozen ? 'frozen' : size === 'S' ? qtyText(p.stockQty) : `${qtyText(p.stockQty)} ${p.unitAbbr}`}</span>
          </span>
        )}
      </motion.button>
      <button type="button" tabIndex={-1} aria-label={`About ${p.name}`} onClick={onInfo}
        className="absolute right-1.5 top-1.5 grid h-6 w-6 place-items-center rounded-full text-faint hover:bg-muted hover:text-foreground">
        <Info size={14} />
      </button>
    </div>
  )
}
