import { useSuspenseQuery } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { qtyText } from '@/lib/money'
import { slots } from '@/plugins/slots'
import { configQuery } from '../api'
import { grouped } from '../cart/groups'
import { useTillApi } from '../TillUi'
import { useTill } from '../TillProvider'
import { CartGroup } from './CartGroup'
import { CartLineRow } from './CartLineRow'
import { HeldStrip } from './HeldSales'
import { PostSaleChange } from './PostSaleChange'

export function CartPanel() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state } = useTill()
  const till = useTillApi()
  const list = useRef<HTMLDivElement>(null)
  const prev = useRef(state.lines.length)
  // Follow growth only: a freshly scanned line below the fold stays in view, but
  // removing a line never yanks the list around.
  useEffect(() => {
    if (state.lines.length > prev.current && list.current) list.current.scrollTop = list.current.scrollHeight
    prev.current = state.lines.length
  }, [state.lines.length])
  const items = state.lines.reduce((s, l) => s + l.qty, 0)
  return (
    <>
      <header className="flex items-center justify-between px-4 pb-2.5 pt-3.5 font-bold">
        <span>Current sale</span>
        <small className="font-medium text-muted-foreground">{qtyText(items)} items</small>
      </header>
      <PostSaleChange />
      <div ref={list} className="flex min-h-0 flex-1 flex-col overflow-auto px-2.5 pb-2.5">
        {state.lines.length
          ? grouped(state.lines).map((x) => (x.kind === 'line'
            ? <CartLineRow key={x.line.key} line={x.line} sym={cfg.symbol} />
            : <CartGroup key={`g${x.id}`} id={x.id} label={x.label} count={x.count} lines={x.lines} sym={cfg.symbol} />))
          : <p className="m-auto p-8 text-center text-faint">Cart is empty. Scan or tap a product to start.</p>}
      </div>
      {slots.tillCartHints.length > 0 && state.lines.length > 0 && (
        <div className="flex flex-col gap-1.5 px-2.5 pb-2 empty:hidden">
          {slots.tillCartHints.map(({ key, Hint }) => <Hint key={key} till={till} lines={state.lines} />)}
        </div>
      )}
      <HeldStrip />
    </>
  )
}
