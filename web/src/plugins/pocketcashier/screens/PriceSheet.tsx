import { useState } from 'react'
import type { CopyProduct, Lot } from '@/api/types.gen'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { lotExpired, lotLabel } from '@/features/till/cart/lines'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'

/** Which price? — the PC's lot pick as a sheet: read the sticker, tap its lot. An
 * expired lot asks once more (writing it off stays on the PC). */
export function PriceSheet({ product, lots, sym, onPick, onClose }: {
  product: CopyProduct
  lots: Lot[]
  sym: string
  onPick: (lot: Lot) => void
  onClose: () => void
}) {
  const [expired, setExpired] = useState<Lot | null>(null)
  return (
    <Sheet open onOpenChange={(o) => !o && (expired ? setExpired(null) : onClose())}>
      <SheetContent side="bottom" className="max-h-[85dvh] gap-3 overflow-y-auto p-4">
        <SheetHeader className="p-0">
          <SheetTitle>{expired ? 'This lot is expired' : 'Which price?'}</SheetTitle>
          <SheetDescription>
            {expired ? `${product.name} — ${lotLabel(expired)}. Sell it anyway?` : `${product.name} has lots at different prices or an expired lot. Read the sticker on the package.`}
          </SheetDescription>
        </SheetHeader>
        {expired ? (
          <div className="flex gap-2">
            <button type="button" onClick={() => setExpired(null)} className="flex-1 rounded-[var(--radius)] bg-muted p-3 text-sm font-bold">Back</button>
            <button type="button" onClick={() => onPick(expired)} className="flex-1 rounded-[var(--radius)] bg-brand p-3 text-sm font-extrabold text-brand-foreground">Sell anyway</button>
          </div>
        ) : (
          <ul className="flex flex-col gap-2">
            {lots.map((l) => (
              <li key={l.batchId}>
                <button type="button" onClick={() => (lotExpired(l) ? setExpired(l) : onPick(l))}
                  className={cn('flex w-full items-center gap-3 rounded-[var(--radius-sm)] border border-border bg-muted p-3 text-left', lotExpired(l) && 'border-danger')}>
                  <span className="flex-1">
                    <span className="block font-display text-lg font-bold tabular-nums">{money(sym, l.price)}</span>
                    <span className="text-[12.5px] text-muted-foreground">{lotLabel(l)} · {qtyText(l.qtyRemaining)} left</span>
                  </span>
                  {lotExpired(l) && <span className="rounded-md bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] px-1.5 text-[11px] font-bold text-danger">Expired</span>}
                </button>
              </li>
            ))}
          </ul>
        )}
      </SheetContent>
    </Sheet>
  )
}
