import { useEffect, useState } from 'react'
import { num, short } from '@/lib/money'
import { cn } from '@/lib/utils'
import { TOUCH_ONLY } from './TouchPad'

/** Cash by the note, for touch-screen tills: tap what the customer hands over —
 * 500 once, 100 twice = 700 — each note counting its taps. Typing the amount
 * (or Clear on the keypad) starts the count again. */
export function NoteTray({ notes, amount, onChange }: { notes: number[]; amount: string; onChange: (v: string) => void }) {
  const [taps, setTaps] = useState<Record<number, number>>({})
  const tally = Object.entries(taps).reduce((s, [n, c]) => s + Number(n) * c, 0)
  // The amount changed some other way (typed, cleared, added as a payment): start again.
  useEffect(() => {
    if (Math.abs(num(amount) - tally) > 0.005) setTaps({})
  }, [amount, tally])
  function tap(n: number) {
    const next = { ...taps, [n]: (taps[n] ?? 0) + 1 }
    setTaps(next)
    onChange(String(Object.entries(next).reduce((s, [v, c]) => s + Number(v) * c, 0)))
  }
  return (
    <div role="group" aria-label="Notes handed over" onPointerDown={(e) => e.preventDefault()} className={cn(TOUCH_ONLY, 'grid-cols-4 gap-1.5')}>
      {[...notes].sort((a, b) => a - b).map((n) => (
        <button key={n} type="button" onClick={() => tap(n)} aria-label={`Add a ${n} note`}
          className="relative h-11 rounded-[var(--radius-sm)] border border-border bg-card font-display text-[15px] font-bold tabular-nums shadow-[var(--shadow)] active:scale-95">
          {short(n)}
          {taps[n] ? <span className="absolute -top-1.5 -right-1.5 grid size-5 place-items-center rounded-full bg-brand text-[11px] text-brand-foreground">×{taps[n]}</span> : null}
        </button>
      ))}
    </div>
  )
}
