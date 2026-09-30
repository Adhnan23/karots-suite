import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { ExternalLink } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { OrderRequest, ReceiveRequest, ReceiveResult, StockLine, SupplierOrder, SupplierView } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { configQuery, drawerQuery, errMsg, lockersOf } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { orderQuery } from '../queries'
import { afterSlip, moneySlip } from '../slip'
import { LineEditor } from '@/features/purchasing/LineEditor'
import { billTotals } from '@/features/purchasing/totals'

/** Take in a delivery (walk-in or against an order) or place an order. */
export function StockForm({ mode, supplier, orderId }: { mode: 'receive' | 'order'; supplier: SupplierView; orderId: number | null }) {
  const cfg = useSuspenseQuery(configQuery).data
  const qc = useQueryClient()
  const order = useQuery({ ...orderQuery(orderId ?? 0), enabled: orderId !== null }).data
  const [lines, setLines] = useState<StockLine[] | null>(null)
  const shown = lines ?? (orderId && order ? order.lines : [])
  const [invoice, setInvoice] = useState('')
  const [notes, setNotes] = useState('')
  const [keepRest, setKeepRest] = useState(true)
  const [pay, setPay] = useState('')
  const [payMethod, setPayMethod] = useState<'cash' | 'card' | 'online'>('cash')
  const [payFrom, setPayFrom] = useState('') // '' = my drawer (cash) / not from a shop account (card, online)
  const [disc, setDisc] = useState('')
  const [discType, setDiscType] = useState<'fixed' | 'percent'>('fixed')
  const [wantedBy, setWantedBy] = useState('')
  const drawer = useQuery(drawerQuery).data
  const [busy, setBusy] = useState(false)
  const [placed, setPlaced] = useState<SupplierOrder | null>(null)
  const sym = cfg.symbol
  const { net, bill, total } = billTotals(shown, disc, discType)
  const froms = payMethod === 'cash'
    ? [{ id: '', name: 'My drawer' }, ...(drawer?.lockers ?? []).filter((l) => l.kind !== 'card' && l.kind !== 'bank').map((l) => ({ id: String(l.id), name: `${l.name} (${money(sym, l.balance)})` }))]
    : [{ id: '', name: 'Not from a shop account' }, ...(payMethod === 'online' ? lockersOf(drawer, 'bank') : []).map((l) => ({ id: String(l.id), name: l.name }))]
  const short = orderId !== null && shown.some((l) => num(l.quantity) < num(l.ordered))

  async function submit() {
    if (!shown.length) return void toast.error(mode === 'order' ? 'Add what you want to order' : 'Add what came in')
    if (mode === 'receive' && shown.some((l) => !(num(l.costPrice) > 0))) return void toast.error('Every line needs its cost price')
    setBusy(true)
    try {
      if (mode === 'order') {
        const body: OrderRequest = { notes, lines: shown, wantedBy }
        setPlaced(await api<SupplierOrder>(`/till/suppliers/${supplier.id}/order`, { json: body }))
        toast.success(`Order placed with ${supplier.name}`)
      } else {
        const body: ReceiveRequest = {
          invoiceNo: invoice, notes, lines: shown, keepRemainder: keepRest && short, deferDrawerKick: !cfg.askToPrint,
          discount: disc, discountType: discType, pay: { amount: pay, method: payMethod, lockerId: num(payFrom) },
        }
        const url = orderId ? `/till/orders/${orderId}/receive` : `/till/suppliers/${supplier.id}/receive`
        const r = await api<ReceiveResult>(url, { json: body })
        const paid = num(r.paid)
        afterSlip(cfg, `Received ${money(sym, r.total)} of stock${paid > 0 ? ` · paid ${money(sym, paid)}` : ' on credit'}`, moneySlip(r.receiptId ?? undefined), payMethod === 'cash' && payFrom === '')
        setLines([])
        setPay('')
        setInvoice('')
        setDisc('')
      }
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  if (placed) {
    return (
      <div className="m-auto flex max-w-[320px] flex-col items-center gap-3 p-8 text-center">
        <b className="text-lg">Order #{placed.id} placed</b>
        <p className="text-[13px] text-muted-foreground">It shows under Receive stock when the goods arrive.</p>
        <Button variant="secondary" asChild><a href={`/print/order/${placed.id}`} target="_blank" rel="noreferrer"><ExternalLink size={16} /> Print the order</a></Button>
        <Button variant="ghost" onClick={() => { setPlaced(null); setLines([]) }}>Place another</Button>
      </div>
    )
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <LineEditor lines={shown} onChange={setLines} withPrices={mode === 'receive'} sym={sym} fromOrder={orderId !== null} />
      <footer className="flex flex-col gap-3 border-t border-border p-4">
        {mode === 'receive' ? (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <input aria-label="Supplier invoice no" placeholder="Their invoice no (optional)" value={invoice} onChange={(e) => setInvoice(e.target.value)} className={cn(FIELD, 'min-w-[160px] flex-1')} />
              <span className="flex items-center gap-1.5 text-[13px] font-semibold text-muted-foreground">
                Bill discount
                <input aria-label="Bill discount" inputMode="decimal" placeholder="0" value={disc} onChange={(e) => setDisc(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'w-20 text-right tabular-nums')} />
                <Seg label="Bill discount type" value={discType} onChange={setDiscType} options={[['fixed', sym], ['percent', '%']]} />
              </span>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <input aria-label="Pay now" inputMode="decimal" placeholder="Pay now (blank = on credit)" value={pay} onChange={(e) => setPay(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'min-w-[150px] flex-1 tabular-nums')} />
              <Seg label="Pay by" value={payMethod} onChange={(m) => { setPayMethod(m); setPayFrom('') }} options={[['cash', 'Cash'], ['card', 'Card'], ['online', 'Online']]} />
              {froms.length > 1 && (
                <select aria-label="Pay from" value={payFrom} onChange={(e) => setPayFrom(e.target.value)} className={cn(FIELD, 'appearance-auto max-w-[200px]')}>
                  {froms.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}
                </select>
              )}
            </div>
            {bill > 0 && <p className="flex justify-between text-[13px] text-muted-foreground tabular-nums">Lines {money(sym, net)} · bill discount −{money(sym, bill)}<b className="text-foreground">{money(sym, total)}</b></p>}
            {short && (
              <label className="flex items-center gap-2 text-[13px] text-muted-foreground">
                <input type="checkbox" className="accent-[var(--brand)]" checked={keepRest} onChange={(e) => setKeepRest(e.target.checked)} />
                Keep what didn't come on order (a new order for the rest)
              </label>
            )}
          </>
        ) : (
          <div className="flex flex-wrap items-center gap-2">
            <input aria-label="Order notes" placeholder="Notes for the supplier (optional)" value={notes} onChange={(e) => setNotes(e.target.value)} className={cn(FIELD, 'min-w-[200px] flex-1')} />
            <label className="flex items-center gap-1.5 text-[13px] font-semibold text-muted-foreground">
              Wanted by <input type="date" aria-label="Wanted by" value={wantedBy} onChange={(e) => setWantedBy(e.target.value)} className={cn(FIELD, 'w-[150px]')} />
            </label>
          </div>
        )}
        <Button size="lg" disabled={busy || !shown.length} onClick={() => void submit()} className="h-12 text-[15px]">
          {mode === 'order' ? `Place order · ${shown.length} item${shown.length === 1 ? '' : 's'}` : `Receive ${money(sym, total)} of stock`}
        </Button>
      </footer>
    </div>
  )
}
