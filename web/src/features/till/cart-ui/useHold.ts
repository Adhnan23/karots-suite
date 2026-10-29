import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useCallback } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { HeldSale } from '@/api/types.gen'
import { dec, num } from '@/lib/money'
import { configQuery, customersQuery, errMsg, heldQuery } from '../api'
import { toHeldBlob } from '../cart/held'
import { saleTotal } from '../cart/totals'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'

export function useHold() {
  const { state, dispatch } = useTill()
  const cfg = useSuspenseQuery(configQuery).data
  const customers = useQuery(customersQuery).data ?? []
  const qc = useQueryClient()
  return useCallback(async () => {
    if (!state.lines.length) return void toast.error('Cart is empty')
    const label = customers.find((c) => c.id === state.customerId)?.name ?? state.lines[0].name
    try {
      await api('/till/held', {
        json: {
          label, saleType: state.saleType, customerId: state.customerId ?? undefined,
          discount: String(num(state.billDiscount)), cart: toHeldBlob(state), itemCount: state.lines.length, total: dec(saleTotal(state)),
        },
      })
      dispatch({ type: 'newSale', saleType: cfg.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail' })
      toast.success('Sale held')
      void qc.invalidateQueries({ queryKey: heldQuery.queryKey })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }, [state, customers, cfg.defaultSaleType, dispatch, qc])
}

/** Resume a held sale. Refuses while another sale is in the cart (replacing it
 * would lose a live sale). */
export function useResume() {
  const { state, dispatch } = useTill()
  const ui = useTillUi()
  const qc = useQueryClient()
  return useCallback(async (h: HeldSale) => {
    if (state.lines.length) return void toast.error('Finish or hold the current sale first')
    dispatch({ type: 'resume', held: h })
    ui.close()
    await api(`/till/held/${h.id}`, { method: 'DELETE' }).catch(() => undefined)
    void qc.invalidateQueries({ queryKey: heldQuery.queryKey })
  }, [state.lines.length, dispatch, ui, qc])
}
