import { useQuery } from '@tanstack/react-query'
import { useCallback } from 'react'
import type { TillProduct } from '@/api/types.gen'
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
    if (lots.length) {
      ui.open({ kind: 'pricePick', product: p, lots })
      return
    }
    const isNew = !state.lines.some((l) => l.productId === p.id && l.batchId === 0 && !l.isService)
    dispatch({ type: 'add', product: p, lots })
    if (p.suggestion && isNew) ui.open({ kind: 'suggestion', product: p })
  }, [opts, state.lines, dispatch, ui])
}
