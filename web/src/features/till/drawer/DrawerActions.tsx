import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Archive, Lock as LockIcon } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { slots } from '@/plugins/slots'
import { configQuery, errMsg, heldQuery } from '../api'
import { Kbd } from '../bits'
import { useHold } from '../cart-ui/useHold'
import { useTillUi } from '../TillUi'
import { CHIP } from '../ui'

/** The till's action row (bottom of the catalogue): drawer moves + plugin actions. */
export function DrawerActions() {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const hold = useHold()
  const held = useQuery(heldQuery).data ?? []
  const noSale = () =>
    api('/till/drawer/no-sale', { method: 'POST' }).then(() => toast.success('Drawer opened')).catch((e) => toast.error(errMsg(e)))
  return (
    <div className="flex flex-wrap items-center gap-2" role="toolbar" aria-label="Till actions">
      <button type="button" className={CHIP} onClick={() => void hold()}>Hold <Kbd>F4</Kbd></button>
      <button type="button" className={CHIP} onClick={() => ui.open({ kind: 'held' })}>Held ({held.length})</button>
      <button type="button" className={CHIP} onClick={() => ui.open({ kind: 'deposit' })}>
        <span className="text-lg leading-none text-success">↓</span> Deposit
      </button>
      <button type="button" className={CHIP} onClick={() => ui.open({ kind: 'withdraw' })}>
        <span className="text-lg leading-none text-danger">↑</span> Withdraw
      </button>
      {cfg.openCashDrawer && (
        <button type="button" className={CHIP} onClick={noSale} title="Open the cash drawer without a sale">
          <Archive size={14} /> Open drawer
        </button>
      )}
      {slots.tillActions.map((a) => (
        <button key={a.key} type="button" className={CHIP} onClick={() => ui.open({ kind: 'action', key: a.key })}>{a.label}</button>
      ))}
      <button type="button" className={`${CHIP} ml-auto`} onClick={() => ui.open({ kind: 'close' })}>
        <LockIcon size={14} /> Close till
      </button>
    </div>
  )
}
