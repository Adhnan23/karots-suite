import type { CopyProduct, Lot } from '@/api/types.gen'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { linePacks } from '@/features/till/cart/packs'
import { unitPriceFor } from '@/features/till/cart/lines'
import type { SaleType } from '@/features/till/cart/types'
import { money, num } from '@/lib/money'

/** Loose or by the pack? — the PC's pack pick as a sheet; each row adds at once. */
export function PackSheet({ product, lot, saleType, sym, onUnits, onClose }: {
  product: CopyProduct
  lot?: Lot
  saleType: SaleType
  sym: string
  onUnits: (units: number) => void
  onClose: () => void
}) {
  const unit = lot && num(lot.price) > 0 && saleType !== 'wholesale' ? num(lot.price) : unitPriceFor(product, saleType)
  const ways = [
    { key: 'unit', title: `1 ${product.unitAbbr}`, units: 1, price: unit },
    ...linePacks(product, saleType).map((k) => ({
      key: String(k.id), title: `1 ${k.name} ×${k.size}`, units: k.size,
      price: k.price > 0 && k.price < k.size * unit ? k.price : k.size * unit,
    })),
  ]
  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="bottom" className="gap-3 p-4">
        <SheetHeader className="p-0">
          <SheetTitle>{product.name}</SheetTitle>
          <SheetDescription>Selling it loose or by the pack? Change the count from the cart.</SheetDescription>
        </SheetHeader>
        <ul className="flex flex-col gap-2">
          {ways.map((w) => (
            <li key={w.key}>
              <button type="button" onClick={() => onUnits(w.units)}
                className="flex w-full items-center gap-3 rounded-[var(--radius-sm)] border border-border bg-muted p-3 text-left">
                <span className="flex-1 text-[15px] font-bold">{w.title}</span>
                <span className="font-display text-lg font-bold tabular-nums">{money(sym, w.price)}</span>
              </button>
            </li>
          ))}
        </ul>
      </SheetContent>
    </Sheet>
  )
}
