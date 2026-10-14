import { useQueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { TillProduct } from '@/api/types.gen'
import { slots } from '@/plugins/slots'
import { errMsg, searchQuery } from '../api'
import { parseEntry } from '../cart/entry'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { useAddProduct } from './useAddProduct'

const byBarcode = (code: string) => api<TillProduct>(`/till/products/barcode/${encodeURIComponent(code)}`)
const notFound = (e: unknown) => e instanceof ApiError && e.status === 404

/** Enter in the scan/search box. The box is cleared IMMEDIATELY so a scanner's next
 * code never lands on top of this one while the lookup is in flight. */
export function useEntry() {
  const ui = useTillUi()
  const { dispatch } = useTill()
  const add = useAddProduct()
  const qc = useQueryClient()
  return useCallback(async (raw: string) => {
    const e = parseEntry(raw)
    if (e.kind === 'empty') return
    ui.setSearch('')
    const text = raw.trim()
    const claim = slots.tillActions.find((a) => a.claimsText?.(text))
    if (claim) return ui.open({ kind: 'action', key: claim.key, text })
    if (e.kind === 'quick') {
      // Phone rush handoff: SET the qty (idempotent on re-scan), no lot prompt.
      if (e.qty <= 0) return void toast.error('Quick-sale code has no quantity')
      try {
        const p = await byBarcode(e.barcode)
        dispatch({ type: 'quickSet', product: p, qty: e.qty })
        toast.success(`${p.name} × ${e.qty}`)
      } catch (err) {
        toast.error(notFound(err) ? `No item with barcode ${e.barcode} — add it manually` : errMsg(err, 'Lookup failed'))
      }
      return
    }
    if (e.kind === 'barcode') {
      try {
        return add(await byBarcode(e.code))
      } catch (err) {
        if (!notFound(err)) return void toast.error(errMsg(err, 'Lookup failed'))
      }
    }
    const q = e.kind === 'barcode' ? e.code : e.q
    const page = await qc.fetchQuery(searchQuery(q, true))
    if (page.items.length === 1) return add(page.items[0])
    if (page.items.length === 0) {
      const digits = /^\d+$/.test(q)
      return ui.open({ kind: 'quickItem', barcode: digits ? q : '', name: digits ? '' : q })
    }
    ui.setSearch(q) // several matches: leave them on screen to tap
  }, [ui, dispatch, add, qc])
}
