import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { addRequest, askAgain, similarQuery } from './api'
import { askLabel } from './labels'
import type { RequestRow } from './types'

/** The till's "Note request": what a customer asked for that the shop doesn't
 * carry. Matching requests show while typing — Asked again adds this customer
 * to one instead of a duplicate; won't-stock ones say why. */
export function NotePanel({ onClose, text = '' }: { onClose: () => void; text?: string }) {
  const [note, setNote] = useState(text)
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [busy, setBusy] = useState(false)
  const q = useDebounced(note.trim(), 250)
  const similar = useQuery(similarQuery(q)).data ?? []
  const contact = { name: name.trim(), phone: phone.trim() }

  async function run(p: Promise<unknown>, done: string) {
    setBusy(true)
    try {
      await p
      toast.success(done)
      onClose()
    } catch (e) {
      toast.error(errMsg(e, 'Could not save the request'))
    } finally {
      setBusy(false)
    }
  }
  const save = () => note.trim() && !busy && run(addRequest(note.trim(), contact), 'Request noted')
  const again = (r: RequestRow) => run(askAgain(r.id, contact), `Added to “${r.note}”`)

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>Customer request</DialogTitle>
          <DialogDescription>Something a customer asked for that we don’t stock yet.</DialogDescription>
        </DialogHeader>
        <form className="grid grid-cols-2 gap-3" onSubmit={(e) => { e.preventDefault(); void save() }}>
          <textarea autoFocus aria-label="What did they ask for?" placeholder="What did they ask for? e.g. red umbrella, 55-inch TV…" rows={3}
            autoComplete="off" value={note} onChange={(e) => setNote(e.target.value)} className={cn(FIELD, 'col-span-2 h-auto py-2')} />
          {similar.length > 0 && (
            <div className="col-span-2 grid gap-1.5">
              <div className="text-xs font-medium text-muted-foreground">Asked before</div>
              {similar.map((r) => (
                <div key={r.id} className="flex items-center gap-2 rounded-[var(--radius)] border border-border bg-muted/40 px-3 py-2">
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-sm font-medium">{r.note}</div>
                    <div className="truncate text-xs text-muted-foreground">
                      {r.status === 'declined' ? `Won’t stock: ${r.reason}` : askLabel(r.askCount, r.lastAsked)}
                    </div>
                  </div>
                  <Button type="button" size="sm" variant="secondary" disabled={busy} onClick={() => again(r)}>Asked again</Button>
                </div>
              ))}
            </div>
          )}
          <input aria-label="Customer name" placeholder="Customer name (optional)" autoComplete="off" value={name}
            onChange={(e) => setName(e.target.value)} className={FIELD} />
          <input aria-label="Phone" placeholder="Phone (optional)" inputMode="tel" autoComplete="off" value={phone}
            onChange={(e) => setPhone(e.target.value)} className={FIELD} />
          <DialogFooter className="col-span-2">
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy || !note.trim()}>Save request</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
