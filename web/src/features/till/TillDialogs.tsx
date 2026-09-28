import { slots } from '@/plugins/slots'
import { CloseRegister } from './drawer/CloseRegister'
import { MoneyMove } from './drawer/MoneyMove'
import { SlipPrompt } from './drawer/SlipPrompt'
import { useTillApi, useTillUi } from './TillUi'

/** Renders whichever till dialog is open. Tasks 10–13 add their cases here. */
export function TillDialogs() {
  const ui = useTillUi()
  const till = useTillApi()
  const d = ui.dialog
  if (!d) return null
  switch (d.kind) {
    case 'withdraw':
    case 'deposit':
      return <MoneyMove kind={d.kind} />
    case 'close':
      return <CloseRegister logout={d.logout} />
    case 'slip':
      return <SlipPrompt receiptId={d.receiptId} />
    case 'action': {
      const a = slots.tillActions.find((x) => x.key === d.key)
      return a ? <a.Panel till={till} onClose={ui.close} /> : null
    }
    default:
      return null
  }
}
