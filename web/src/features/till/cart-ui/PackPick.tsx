import { useSuspenseQuery } from '@tanstack/react-query'
import type { KeyboardEvent } from 'react'
import type { Lot, TillProduct } from '@/api/types.gen'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, num } from '@/lib/money'
import { configQuery } from '../api'
import { Kbd } from '../bits'
import { unitPriceFor } from '../cart/lines'
import { linePacks } from '../cart/packs'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'

/** "How are you selling it?" — one unit, or one of its packs (keys 1–9, Enter =
 * one unit). Each button adds straight away; more of either from the cart line. */
export function PackPick({ product, lot, lots }: { product: TillProduct; lot?: Lot; lots: Lot[] }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const ui = useTillUi()
  const { state, dispatch } = useTill()
  const unit = lot && num(lot.price) > 0 && state.saleType !== 'wholesale' ? num(lot.price) : unitPriceFor(product, state.saleType)
  const ways = [
    { key: 'unit', title: `1 ${product.unitAbbr}`, units: 1, price: unit },
    ...linePacks(product, state.saleType).map((k) => ({
      key: String(k.id), title: `1 ${k.name} ×${k.size}`, units: k.size,
      price: k.price > 0 && k.price < k.size * unit ? k.price : k.size * unit,
    })),
  ]
  function add(units: number) {
    ui.close()
    dispatch({ type: 'add', product, lot, lots, units })
    if (product.suggestion) ui.open({ kind: 'suggestion', product })
  }
  function onKey(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      add(1)
      return
    }
    const n = Number.parseInt(e.key, 10)
    if (n >= 1 && n <= ways.length) {
      e.preventDefault()
      add(ways[n - 1].units)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent onKeyDown={onKey} className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>{product.name}</DialogTitle>
          <DialogDescription>Selling it loose or by the pack? Add more of either on the cart line.</DialogDescription>
        </DialogHeader>
        <ol className="flex flex-col gap-2">
          {ways.map((w, i) => (
            <li key={w.key}>
              <button type="button" autoFocus={i === 0} onClick={() => add(w.units)}
                className="flex w-full items-center gap-3 rounded-[var(--radius-sm)] border border-border bg-muted p-3 text-left hover:border-brand focus-visible:border-brand focus-visible:outline-none">
                <Kbd>{i + 1}</Kbd>
                <span className="flex-1 text-[15px] font-bold">{w.title}</span>
                <span className="font-display text-lg font-bold tabular-nums">{money(sym, w.price)}</span>
              </button>
            </li>
          ))}
        </ol>
      </DialogContent>
    </Dialog>
  )
}
