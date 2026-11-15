import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { refreshStock, unfreeze } from './api'

/** "How many are on the shelf now?" — added to the counted stock; from then on
 * sales reduce it. A small dialog over whatever screen asked (a full cart stays). */
export function UnfreezeDialog({ id, name, unit, onDone, onClose }: {
  id: number
  name: string
  unit?: string
  onDone?: (stock: number) => void
  onClose: () => void
}) {
  const qc = useQueryClient()
  const [qty, setQty] = useState('')
  const [busy, setBusy] = useState(false)
  async function save(e: { preventDefault(): void }) {
    e.preventDefault()
    if (qty === '') return
    setBusy(true)
    try {
      const p = await unfreeze(id, qty)
      toast.success(`${name} unfrozen: ${qtyText(p.stockQty)} ${p.unitAbbr} in stock`)
      refreshStock(qc)
      onDone?.(num(p.stockQty))
      onClose()
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-[400px]">
        <form onSubmit={save} autoComplete="off" className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Unfreeze {name}</DialogTitle>
            <DialogDescription>How many are on the shelf now? They're added to the counted stock, and sales count from now on.</DialogDescription>
          </DialogHeader>
          <label className="flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground">
            On the shelf now{unit && ` (${unit})`}
            <input autoFocus aria-label="On the shelf now" inputMode="decimal" value={qty} placeholder="0"
              onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))}
              className={cn(FIELD, 'h-12 font-display text-xl font-bold tabular-nums')} />
          </label>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Not now</Button>
            <Button type="submit" disabled={busy || qty === ''}>Unfreeze</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
