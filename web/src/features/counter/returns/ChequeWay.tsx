/** The sale was paid by a cheque that hasn't cleared: give it back, or refund cash and
 *  keep it (RET-17, user 2026-11-18). Shown by every return screen; nothing is
 *  refunded until one is picked. */
export function ChequeWay({ way, onPick, customer }: { way: 'back' | 'cash' | ''; onPick: (w: 'back' | 'cash') => void; customer: string }) {
  return (
    <div role="radiogroup" aria-label="The cheque hasn't cleared" className="flex flex-col gap-1.5">
      <p className="text-[13px] text-muted-foreground">The customer paid by cheque and it hasn't cleared yet.</p>
      <div className="grid grid-cols-2 gap-1.5">
        {([['back', 'Give the cheque back'], ['cash', 'Refund cash, keep the cheque']] as const).map(([k, label]) => (
          <button key={k} type="button" role="radio" aria-checked={way === k} onClick={() => onPick(k)}
            className="rounded-[var(--radius-sm)] border border-border px-3 py-2 text-[13px] font-semibold aria-checked:border-brand aria-checked:bg-brand aria-checked:text-brand-foreground">{label}</button>
        ))}
      </div>
      {way === 'cash' && <small className="text-muted-foreground">If the cheque bounces, its amount goes on {customer}'s account.</small>}
    </div>
  )
}
