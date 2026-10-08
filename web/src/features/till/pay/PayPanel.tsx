import { useSuspenseQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { configQuery } from '../api'
import { Kbd, Seg } from '../bits'
import { useHold } from '../cart-ui/useHold'
import { useCheckoutCtx } from '../checkout/CheckoutProvider'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { ChangeBanner } from './ChangeBanner'
import { CustomerPicker } from './CustomerPicker'
import { Summary } from './Summary'
import { Tender } from './Tender'
import { TotalStrip } from './TotalStrip'

export function PayPanel() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const co = useCheckoutCtx()
  const hold = useHold()
  const ui = useTillUi()
  // Is the Total + change block fully on screen? On a short kiosk screen it scrolls
  // away, and the pinned TotalStrip stands in for it.
  const scroller = useRef<HTMLDivElement>(null)
  const totals = useRef<HTMLDivElement>(null)
  const [totalsSeen, setTotalsSeen] = useState(true)
  useEffect(() => {
    if (!totals.current) return
    const io = new IntersectionObserver(([e]) => setTotalsSeen(e.intersectionRatio > 0.98), { root: scroller.current, threshold: [0.98, 1] })
    io.observe(totals.current)
    return () => io.disconnect()
  }, [])
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div ref={scroller} className="flex min-h-0 flex-1 flex-col gap-3 overflow-auto p-3.5">
        <div className="flex items-center justify-between">
          <Seg label="Price list" value={state.saleType} options={[['retail', 'Retail'], ['wholesale', 'Wholesale']]}
            onChange={(v) => dispatch({ type: 'saleType', saleType: v })} />
          <span className="text-[12px] text-faint">{cfg.askToPrint ? 'Asks before printing' : 'Prints automatically'}</span>
        </div>
        <CustomerPicker />
        <Tender />
        <div ref={totals} className="flex flex-col gap-3">
          <Summary />
          <ChangeBanner />
        </div>
      </div>
      <div className="flex shrink-0 flex-col gap-2 border-t border-border p-3.5">
        {!totalsSeen && <TotalStrip />}
        <Button className="h-[54px] text-base" disabled={!state.lines.length || co.busy} onClick={() => void co.run()}>
          {co.busy ? 'Completing…' : 'Complete sale'} <Kbd className="border-current bg-transparent text-current">Enter</Kbd>
        </Button>
        <div className="flex gap-2">
          <Button variant="secondary" className="flex-1" onClick={() => void hold()}>Hold <Kbd>F4</Kbd></Button>
          <Button variant="secondary" className="flex-1" onClick={() => (state.lines.length ? ui.open({ kind: 'clearSale' }) : co.newSale())}>New sale</Button>
        </div>
      </div>
    </div>
  )
}
