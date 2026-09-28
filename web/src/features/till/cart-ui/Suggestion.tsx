import { useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import type { TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, num } from '@/lib/money'
import { configQuery } from '../api'
import { unitPriceFor } from '../cart/lines'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'

/** A plugin-proposed markdown (e.g. a clearance price), offered once per new line. */
export function Suggestion({ product }: { product: TillProduct }) {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const ui = useTillUi()
  const [key] = useState(state.lastAddedKey)
  const s = product.suggestion
  if (!s) return null
  const pct = s.discountType === 'percent'
  const unit = unitPriceFor(product, state.saleType)
  const next = Math.max(0, pct ? unit * (1 - num(s.discountValue) / 100) : unit - num(s.discountValue))
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>{s.label || 'Suggested discount'}</DialogTitle>
          <DialogDescription>{s.prompt || 'Apply the suggested discount?'}</DialogDescription>
        </DialogHeader>
        <p className="text-center text-[15px] tabular-nums">
          <s className="text-faint">{money(cfg.symbol, unit)}</s> → <b className="font-display text-2xl">{money(cfg.symbol, next)}</b>
        </p>
        <DialogFooter>
          <Button variant="ghost" onClick={ui.close}>Skip</Button>
          <Button autoFocus onClick={() => {
            dispatch({ type: 'lineDiscount', key, discount: s.discountValue, discountType: pct ? 'percent' : 'fixed' })
            ui.close()
          }}>Apply</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
