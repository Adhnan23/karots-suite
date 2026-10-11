import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { dec, money, qtyText } from '@/lib/money'
import { slots } from '@/plugins/slots'
import { configQuery, customersQuery } from '../api'
import { blankDraft } from '../cart/store'
import { useTillApi } from '../TillUi'
import { useTill } from '../TillProvider'
import { useCheckoutCtx } from './CheckoutProvider'

function Confirm({ title, body, yes, no = 'Cancel', onYes, onNo, extra }: {
  title: string
  body: ReactNode
  yes: string
  no?: string
  onYes: () => void
  onNo?: () => void
  extra?: ReactNode
}) {
  return (
    <Dialog open onOpenChange={(o) => !o && (onNo ?? onYes)()}>
      <DialogContent className="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{body}</DialogDescription>
        </DialogHeader>
        {extra}
        <DialogFooter>
          {onNo && <Button variant="ghost" onClick={onNo}>{no}</Button>}
          <Button autoFocus onClick={onYes}>{yes}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Styled in-app confirms (never the browser's confirm()), one at a time. */
export function Prompts() {
  const co = useCheckoutCtx()
  const cfg = useSuspenseQuery(configQuery).data
  const { state } = useTill()
  const till = useTillApi()
  const name = (useQuery(customersQuery).data ?? []).find((c) => c.id === state.customerId)?.name ?? 'the customer'
  const sym = cfg.symbol
  if (co.consumable) {
    return <Confirm title="Material reads short" body="A job’s material (paper, ink…) reads short on the count. Sell anyway and correct the count up?"
      yes="Sell anyway" onYes={() => co.answer({ type: 'approveConsumables' }, false)} onNo={co.dismiss} />
  }
  const p = co.prompt
  const line = p?.kind === 'oversell' ? state.lines.find((l) => l.key === p.key) : undefined
  if (!p) return null
  switch (p.kind) {
    case 'oversell':
      return <Confirm title="Sell more than the count shows?"
        body={`The count shows ${qtyText(p.stock)} of ${p.name}; you are selling ${qtyText(p.qty)}. If the customer is holding it, the count is corrected up.`}
        yes={`Sell ${qtyText(p.qty)}`} onYes={() => co.answer({ type: 'approveOversell', key: p.key }, false)} onNo={co.dismiss}
        extra={line && slots.tillShortHints.map(({ key, Hint }) => <Hint key={key} till={till} line={line} onDone={co.dismiss} />)} />
    case 'shortfall':
      return p.hasCustomer
        ? <Confirm title={`${money(sym, p.amount)} short`} body={`Put ${money(sym, p.amount)} on ${name}’s account?`} yes="Put on account" no="Take more money"
            onYes={() => co.answer({ type: 'addTender', payment: { ...blankDraft('credit'), amount: dec(p.amount) } }, true)} onNo={co.dismiss} />
        : <Confirm title={`${money(sym, p.amount)} short`} body="Take more money, or pick a customer to sell on account." yes="OK" onYes={co.dismiss} />
    case 'overcovered':
      return <Confirm title="On account covers more than the bill"
        body={`The on-account line (${money(sym, p.amount)}) plus the money taken is more than the total. Remove the on-account line?`}
        yes="Remove on-account" onYes={() => co.answer({ type: 'removeCreditTenders' }, true)} onNo={co.dismiss} />
    case 'overlimit':
      return <Confirm title="Over the credit limit"
        body={`This puts ${money(sym, p.amount)} on account — ${money(sym, p.over)} over ${name}’s available credit. Approve this one sale?`}
        yes="Approve" onYes={() => co.answer({ type: 'approveOverLimit' }, true)} onNo={co.dismiss} />
  }
}
