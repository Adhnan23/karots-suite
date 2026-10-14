import { useQuery } from '@tanstack/react-query'
import { useCallback } from 'react'
import type { TillProduct } from '@/api/types.gen'
import { num } from '@/lib/money'
import { slots } from '@/plugins/slots'
import { priceOptionsQuery } from '../api'
import { lotsFor } from '../cart/lines'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'

/** Add one unit of p: ask "which price?" first when its lots disagree or the lot
 * that would sell is expired; offer a plugin sale suggestion on a NEW line. A
 * plugin menu root that claims p opens instead (e.g. a job that needs options). */
export function useAddProduct() {
  const { state, dispatch } = useTill()
  const ui = useTillUi()
  const opts = useQuery(priceOptionsQuery).data
  return useCallback((p: TillProduct) => {
    const claim = slots.tillMenu.find((m) => m.claims?.(p))
    if (claim) {
      ui.setSearch('')
      ui.setStack([])
      ui.setMenu(claim.key, p)
      return
    }
    const lots = lotsFor(opts, p.id)
    // A pack's own barcode adds that pack; otherwise a product sold by the pack asks how.
    const scanned = p.packs.find((k) => k.id === p.scannedPackId)
    const units = scanned ? num(scanned.size) : undefined
    if (lots.length) {
      ui.open({ kind: 'pricePick', product: p, lots, units })
      return
    }
    if (!scanned && p.packs.length && !p.isService) {
      ui.open({ kind: 'packPick', product: p, lots })
      return
    }
    const isNew = !state.lines.some((l) => l.productId === p.id && l.batchId === 0 && !l.isService)
    dispatch({ type: 'add', product: p, lots, units })
    if (p.suggestion && isNew) ui.open({ kind: 'suggestion', product: p })
  }, [opts, state.lines, dispatch, ui])
}
