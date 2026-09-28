import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useEffect, useState, type KeyboardEvent } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { Lot, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, errMsg, priceOptionsQuery } from '../api'
import { Kbd } from '../bits'
import { lotExpired, lotLabel } from '../cart/lines'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'

/** The cashier reads the sticker: pick the lot (1–9, Enter = the FEFO one). An
 * expired lot asks again: sell anyway / write it off / back. */
export function PricePick({ product, lots: cached }: { product: TillProduct; lots: Lot[] }) {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const { dispatch } = useTill()
  const qc = useQueryClient()
  const [lots, setLots] = useState(cached)
  const [expired, setExpired] = useState<Lot | null>(null)

  function commit(lot: Lot, all: Lot[] = lots) {
    ui.close()
    dispatch({ type: 'add', product, lot, lots: all })
    if (product.suggestion) ui.open({ kind: 'suggestion', product })
  }
  const pick = (lot: Lot) => (lotExpired(lot) ? setExpired(lot) : commit(lot))

  // Show the cached lots at once (a rush can't wait), then correct from the server
  // so nobody picks a lot another lane just emptied.
  useEffect(() => {
    let live = true
    api<Lot[]>(`/till/products/${product.id}/lots`).then((fresh) => {
      const f = fresh.filter((l) => num(l.qtyRemaining) > 0)
      if (!live || !f.length) return
      if (new Set(f.map((l) => l.price)).size === 1 && !lotExpired(f[0])) commit(f[0], f)
      else setLots(f)
    }).catch(() => undefined)
    return () => { live = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [product.id])

  async function writeOff() {
    if (!expired) return
    try {
      await api('/till/write-off', { json: { productId: product.id, batchId: expired.batchId, quantity: expired.qtyRemaining, note: `expired ${expired.expiryDate}` } })
      toast.success('Expired stock written off')
      ui.close()
      void qc.invalidateQueries({ queryKey: priceOptionsQuery.queryKey })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }

  function onKey(e: KeyboardEvent) {
    if (expired) return // buttons handle it; Esc goes back via onOpenChange
    if (e.key === 'Enter') {
      e.preventDefault()
      pick(lots[0])
      return
    }
    const n = Number.parseInt(e.key, 10)
    if (n >= 1 && n <= lots.length) {
      e.preventDefault()
      pick(lots[n - 1])
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && (expired ? setExpired(null) : ui.close())}>
      <DialogContent onKeyDown={onKey} className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{expired ? 'This lot is expired' : 'Which price?'}</DialogTitle>
          <DialogDescription>
            {expired
              ? `${product.name} — ${lotLabel(expired)}. Sell it anyway, or write it off instead?`
              : `${product.name} has lots at different prices or an expired lot. Read the sticker on the package.`}
          </DialogDescription>
        </DialogHeader>
        {!expired ? (
          <ol className="flex flex-col gap-2">
            {lots.map((l, i) => (
              <li key={l.batchId}>
                <button type="button" onClick={() => pick(l)}
                  className={cn('flex w-full items-center gap-3 rounded-[var(--radius-sm)] border border-border bg-muted p-3 text-left hover:border-brand', lotExpired(l) && 'border-danger')}>
                  <Kbd>{i + 1}</Kbd>
                  <span className="flex-1">
                    <span className="font-display text-lg font-bold tabular-nums">{money(cfg.symbol, l.price)}</span>
                    <span className="ml-2 text-[13px] text-muted-foreground">{lotLabel(l)} · {qtyText(l.qtyRemaining)} left</span>
                  </span>
                  {lotExpired(l) && <span className="rounded-md bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] px-1.5 text-[11px] font-bold text-danger">Expired</span>}
                </button>
              </li>
            ))}
          </ol>
        ) : (
          <DialogFooter>
            <Button variant="ghost" onClick={() => setExpired(null)}>Back</Button>
            <Button variant="destructive" onClick={writeOff}>Write it off</Button>
            <Button autoFocus onClick={() => commit(expired)}>Sell anyway</Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
