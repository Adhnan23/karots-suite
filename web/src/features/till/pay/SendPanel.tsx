import { Button } from '@/components/ui/button'
import { Seg, Kbd } from '../bits'
import { useHold } from '../cart-ui/useHold'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { CustomerPicker } from './CustomerPicker'
import { Summary } from './Summary'

/** A sales person's Pay panel: the bill, and Send to cashier instead of taking money. */
export function SendPanel() {
  const { state, dispatch } = useTill()
  const hold = useHold()
  const ui = useTillUi()
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-auto p-3.5">
        <Seg label="Price list" value={state.saleType} options={[['retail', 'Retail'], ['wholesale', 'Wholesale']]}
          onChange={(v) => dispatch({ type: 'saleType', saleType: v })} />
        <CustomerPicker />
        <Summary />
      </div>
      <div className="flex shrink-0 flex-col gap-2 border-t border-border p-3.5">
        <Button className="h-[54px] text-base" disabled={!state.lines.length} onClick={() => void hold(true)}>Send to cashier</Button>
        <div className="flex gap-2">
          <Button variant="secondary" className="flex-1" onClick={() => void hold()}>Hold <Kbd>F4</Kbd></Button>
          <Button variant="secondary" className="flex-1" disabled={!state.lines.length} onClick={() => ui.open({ kind: 'clearSale' })}>Clear</Button>
        </div>
      </div>
    </div>
  )
}
