import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useCallback, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { SaleResult } from '@/api/types.gen'
import { num } from '@/lib/money'
import { slots } from '@/plugins/slots'
import { configQuery, customersQuery, drawerQuery, errMsg, lockersOf, useRefreshTill } from '../api'
import { buildSaleRequest, checkoutGate, type Prompt } from '../cart/checkout'
import { tillReducer, type Action } from '../cart/store'
import type { SaleType, TillState } from '../cart/types'
import { printSale } from '../print'
import { useTill } from '../TillProvider'

export function useCheckout() {
  const { state, dispatch } = useTill()
  const cfg = useSuspenseQuery(configQuery).data
  const drawer = useQuery(drawerQuery).data
  const customers = useQuery(customersQuery).data
  const refresh = useRefreshTill()
  const [prompt, setPrompt] = useState<Prompt | null>(null)
  const [consumable, setConsumable] = useState(false)
  const [done, setDone] = useState<SaleResult | null>(null)
  const [busy, setBusy] = useState(false)
  const inFlight = useRef(false)
  const saleType: SaleType = cfg.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail'

  /** Complete the sale. Re-entered with the next state after each prompt answer. */
  const run = useCallback(async (s: TillState = state, confirmed = false) => {
    if (inFlight.current) return
    const customer = (customers ?? []).find((c) => c.id === s.customerId) ?? null
    const gate = checkoutGate(s, {
      customer, canManageCredit: cfg.canManageCredit, confirmed,
      accountKinds: { card: lockersOf(drawer, 'card').length > 0, online: lockersOf(drawer, 'bank').length > 0 },
    })
    if ('prompt' in gate) return setPrompt(gate.prompt)
    if ('error' in gate) return void toast.error(gate.error)
    inFlight.current = true
    setBusy(true)
    try {
      for (const guard of slots.tillBeforeCheckout) {
        const msg = await guard(s.lines)
        if (msg) return void toast.error(msg)
      }
      const res = await api<SaleResult>('/till/sales', { json: buildSaleRequest(s, gate.payments, { deferDrawerKick: !cfg.askToPrint }) })
      void Promise.allSettled(slots.tillAfterSale.map((fn) => fn({ saleId: res.id, lines: s.lines })))
      if (cfg.askToPrint) {
        setDone(res)
      } else {
        // No modal: print in the background (drawer pulse merged into the ONE job)
        // and reset straight away; the change survives as a banner.
        toast.success('Sale complete')
        void printSale(res.id, true)
        dispatch({ type: 'newSale', saleType, change: num(res.changeGiven) })
      }
      void refresh()
    } catch (e) {
      // A job's material reads short: offer found-at-till once per unapproved line.
      if (e instanceof ApiError && e.code === 'CONSUMABLE_SHORT' && s.lines.some((l) => (l.isService || l.components.length > 0) && !l.allowOversell)) {
        setConsumable(true)
        return
      }
      toast.error(errMsg(e, 'Sale failed'))
    } finally {
      inFlight.current = false
      setBusy(false)
    }
  }, [state, customers, cfg, drawer, dispatch, refresh, saleType])

  /** Resolve a prompt: apply the action (if any) and re-run on the NEXT state. */
  const answer = useCallback((action: Action | null, confirmed: boolean) => {
    const next = action ? tillReducer(state, action) : state
    if (action) dispatch(action)
    setPrompt(null)
    setConsumable(false)
    void run(next, confirmed)
  }, [state, dispatch, run])

  const dismiss = useCallback(() => {
    setPrompt(null)
    setConsumable(false)
  }, [])

  const newSale = useCallback(() => {
    setDone(null)
    dispatch({ type: 'newSale', saleType })
    document.getElementById('till-search')?.focus()
  }, [dispatch, saleType])

  return { run, answer, dismiss, newSale, busy, prompt, consumable, done }
}
