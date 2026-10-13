import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { FIELD } from '@/features/till/ui'
import { num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import type { Alt } from './types'

/** A swap to another unit can't keep the line's quantity (1 bag ≠ 1 kg): ask how
 * many of the alternative. Cancel leaves the line as it was. */
export function AskQty({ alt, onSwap, onCancel }: { alt: Alt; onSwap: (qty: number) => void; onCancel: () => void }) {
  const [v, setV] = useState('')
  const q = num(v)
  const over = q > num(alt.stock)
  return (
    <Dialog open onOpenChange={(o) => !o && onCancel()}>
      <DialogContent className="sm:max-w-[380px]">
        <form onSubmit={(e) => { e.preventDefault(); if (q > 0) onSwap(q) }} autoComplete="off" className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>How many {alt.unit} of {alt.name}?</DialogTitle>
            <DialogDescription>It's sold by the {alt.unit}, so the line's count can't carry over. {qtyText(alt.stock)} {alt.unit} in stock.</DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2">
            <input autoFocus aria-label={`How many ${alt.unit}`} inputMode="decimal" value={v} onChange={(e) => setV(e.target.value.replace(/[^\d.]/g, ''))}
              className={cn(FIELD, 'h-12 flex-1 font-display text-xl font-bold tabular-nums', over && 'border-warning')} />
            <span className="font-semibold text-muted-foreground">{alt.unit}</span>
          </div>
          {over && <small className="-mt-2 text-warning">More than the {qtyText(alt.stock)} {alt.unit} in stock.</small>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onCancel}>Cancel</Button>
            <Button type="submit" disabled={!(q > 0)}>Swap</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
