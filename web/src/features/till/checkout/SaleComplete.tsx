import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Check, Printer } from 'lucide-react'
import { motion } from 'motion/react'
import { Button } from '@/components/ui/button'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery } from '../api'
import { Kbd } from '../bits'
import { notesFor } from '../cart/totals'
import { METHOD_LABEL } from '../pay/Tender'
import { printSale, slipPath } from '../print'
import { SlipPaper } from '../SlipPaper'
import { PANEL } from '../ui'
import { useCheckoutCtx } from './CheckoutProvider'

/** "Ask before printing" mode: the change shows instantly beside the receipt as it
 * will print; Print or start the next sale. */
export function SaleComplete() {
  const co = useCheckoutCtx()
  const cfg = useSuspenseQuery(configQuery).data
  const denoms = (useQuery(drawerQuery).data?.denominations ?? []).map((d) => num(d.value))
  const res = co.done
  if (!res) return null
  const sym = cfg.symbol
  const change = num(res.changeGiven)
  const methods = res.methods.map((m) => METHOD_LABEL[m] ?? cfg.tenderMethods.find((t) => t.value === m)?.label ?? m).join(' + ')
  const bal = res.customerBalanceAfter === undefined ? null : num(res.customerBalanceAfter)
  return (
    <div role="dialog" aria-modal="true" aria-labelledby="sale-done-title"
      className="absolute inset-0 z-30 grid place-items-center bg-[color-mix(in_srgb,var(--background)_70%,transparent)] backdrop-blur-[6px]">
      <motion.div initial={{ scale: 0.7, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} transition={{ duration: 0.35, ease: [0.2, 1.4, 0.4, 1] }}
        className={cn(PANEL, 'flex max-h-[calc(100%-32px)] items-stretch gap-5 p-5')}>
        <div className="flex flex-col items-center justify-center gap-2 px-6 py-3 text-center">
          <span className="grid h-16 w-16 place-items-center rounded-full bg-success text-background"><Check size={34} strokeWidth={3} /></span>
          <h3 id="sale-done-title" className="mt-1.5 text-[22px] font-bold">Sale {res.receiptNo} complete</h3>
          <p className="text-muted-foreground tabular-nums">
            {money(sym, res.total)} · {methods}
            {num(res.roundingToAccount) > 0 && ` · ${money(sym, res.roundingToAccount)} to account`}
          </p>
          {change > 0 && (
            <>
              <p className="mt-2 text-muted-foreground">Give change</p>
              <p className="font-display text-[40px] font-extrabold leading-none text-success tabular-nums">{money(sym, change)}</p>
              <p className="text-[13px] text-muted-foreground tabular-nums">{notesFor(change, denoms).map((n) => `${n.qty} × ${qtyText(n.value)}`).join(' · ')}</p>
            </>
          )}
          {bal !== null && (
            <p className="text-[13px] text-muted-foreground">
              {res.customerName} {bal >= 0 ? `now owes ${money(sym, bal)}` : `has ${money(sym, -bal)} advance`}
            </p>
          )}
          <div className="mt-3 flex gap-2">
            <Button variant="secondary" onClick={() => void printSale(res.id)}><Printer size={16} /> Print receipt</Button>
            <Button autoFocus onClick={co.newSale}>Start next sale <Kbd className="border-current bg-transparent text-current">Enter</Kbd></Button>
          </div>
        </div>
        <div className="hidden min-h-0 overflow-auto rounded-[var(--radius-sm)] bg-muted p-3 md:block">
          <SlipPaper path={slipPath.sale(res.id)} />
        </div>
      </motion.div>
    </div>
  )
}
