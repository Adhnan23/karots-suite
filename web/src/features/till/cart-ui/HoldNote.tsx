import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { customersQuery } from '../api'
import { Kbd } from '../bits'
import { heldLabel } from '../cart/held'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'
import { useHold } from './useHold'

/** Hold, with an optional note to find it again (SALE-53). Never in the way:
 * Enter or F4 again holds at once; an empty note keeps today's name. */
export function HoldNote() {
  const ui = useTillUi()
  const hold = useHold()
  const { state } = useTill()
  const customer = (useQuery(customersQuery).data ?? []).find((c) => c.id === state.customerId)?.name
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const name = heldLabel('', customer, state.lines[0]?.name ?? 'Sale')
  const go = () => {
    if (busy) return
    setBusy(true)
    ui.close()
    void hold({ note })
  }
  // F4 again holds wherever the focus is (the till's own F4 is off while a dialog is open).
  const goRef = useRef(go)
  useEffect(() => { goRef.current = go })
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'F4') { e.preventDefault(); goRef.current() } }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>Hold this sale</DialogTitle>
          <DialogDescription>A note helps you find it again. You can leave it empty.</DialogDescription>
        </DialogHeader>
        <input aria-label="Hold note" placeholder="e.g. gone to the ATM" autoComplete="off" autoFocus maxLength={120}
          value={note} onChange={(e) => setNote(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); go() } }}
          className={`${FIELD} h-11 px-3 text-till-body leading-[1.43]`} />
        <p className="text-till-sm text-muted-foreground">
          {note.trim() ? <>Held as <b className="text-foreground">{note.trim()}</b></> : <>Empty: held as <b className="text-foreground">{name}</b></>}
        </p>
        <DialogFooter>
          <Button variant="ghost" onClick={ui.close}>Cancel</Button>
          <Button onClick={go} disabled={busy}>Hold <Kbd>F4</Kbd></Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
