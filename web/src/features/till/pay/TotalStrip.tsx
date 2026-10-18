import { useSuspenseQuery } from '@tanstack/react-query'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery } from '../api'
import { actualChange, balanceDue, changeDue, keptAmount, roundable, saleTotal } from '../cart/totals'
import { useTill } from '../TillProvider'

/** Pinned over Complete sale while the pay panel's own Total / change block is
 * scrolled out of sight (a 1024×768 kiosk): the cashier always sees what they're
 * charging and what's left — same figures as Summary + ChangeBanner. */
export function TotalStrip() {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const { state } = useTill()
  if (!state.lines.length) return null
  const t = saleTotal(state)
  const pays = [...state.tenders, ...(num(state.draft.amount) > 0 ? [state.draft] : [])]
  const due = balanceDue(t, pays)
  const change = changeDue(t, pays)
  const give = actualChange(change, roundable(t, pays) ? keptAmount(state.keep, change) : 0)
  const [label, value, tone] = due > 0 ? ['Still to pay', due, 'text-warning'] : change > 0 ? [t < 0 ? 'Give the customer' : 'Change', give, 'text-success'] : ['Paid in full', 0, 'text-muted-foreground']
  return (
    <div aria-label="Sale total" className="flex items-baseline justify-between gap-3 tabular-nums">
      <span className="whitespace-nowrap text-[13px] text-muted-foreground">{t < 0 ? 'Given' : 'Total'} <b className="ml-1 font-display text-[20px] font-extrabold text-foreground">{money(sym, Math.abs(t))}</b></span>
      <span className="whitespace-nowrap text-right text-[13px] text-muted-foreground">
        {label}{value > 0 && <b className={cn('ml-1.5 font-display text-[17px] font-extrabold', tone)}>{money(sym, value)}</b>}
      </span>
    </div>
  )
}
