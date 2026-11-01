import { useState, type CSSProperties } from 'react'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { FIELD } from '@/features/till/ui'
import { cn } from '@/lib/utils'
import { liveCall } from '../quickItem'

// No browser memory on a shared phone: text boxes shown as dots, never type=password
// (a password manager would offer to save it), non-standard names.
const DOTS = { WebkitTextSecurity: 'disc' } as CSSProperties

/** Change my PIN, as on the PC's account page. The PC keeps PINs, so live only;
 * the phone's copy picks up the new one on its next refresh. */
export function PinSheet({ live, onDone, onClose }: { live: boolean; onDone: () => void; onClose: () => void }) {
  const [pins, setPins] = useState({ currentPin: '', newPin: '', confirmPin: '' })
  const [error, setError] = useState(live ? '' : "The PC isn't reachable — change your PIN when it is")
  const [busy, setBusy] = useState(false)
  const box = (key: keyof typeof pins, label: string) => (
    <input aria-label={label} name={`ks-p-${key}`} type="text" inputMode="numeric" autoComplete="off" autoCorrect="off" spellCheck={false} maxLength={6}
      value={pins[key]} onChange={(e) => { setPins({ ...pins, [key]: e.target.value.replace(/\D/g, '') }); setError('') }}
      placeholder={label} style={DOTS} className={cn(FIELD, 'text-lg tracking-widest')} />
  )
  async function submit() {
    if (pins.newPin !== pins.confirmPin) return setError("The new PINs don't match")
    setBusy(true)
    const r = await liveCall('/auth/pin', pins)
    setBusy(false)
    if ('error' in r) return setError(r.error)
    onDone()
    onClose()
  }
  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="bottom" className="gap-3 p-4">
        <SheetHeader className="p-0">
          <SheetTitle>Change my PIN</SheetTitle>
          <SheetDescription>The same PIN works here and on the PC.</SheetDescription>
        </SheetHeader>
        <form autoComplete="off" className="flex flex-col gap-2.5" onSubmit={(e) => { e.preventDefault(); void submit() }}>
          {box('currentPin', 'Current PIN')}
          {box('newPin', 'New PIN')}
          {box('confirmPin', 'New PIN again')}
          {error && <p role="alert" className="text-sm font-semibold text-destructive">{error}</p>}
          <button type="submit" disabled={busy || !live} className="rounded-[var(--radius)] bg-brand p-3.5 text-base font-extrabold text-brand-foreground disabled:opacity-60">
            {busy ? 'Changing…' : 'Change PIN'}
          </button>
        </form>
      </SheetContent>
    </Sheet>
  )
}
