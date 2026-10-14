import { useEffect, useState } from 'react'
import { num, short } from '@/lib/money'

/** Cash by the note and coin: tap what the customer hands over — 500 once, 100
 * twice = 700 — each counting its taps. Typing the amount (or Clear) starts the
 * count again. Notes are tinted with the palette's chart colours, one per value. */
export function NoteTray({ notes, coins, amount, onChange }: { notes: number[]; coins: number[]; amount: string; onChange: (v: string) => void }) {
  const [taps, setTaps] = useState<Record<number, number>>({})
  const tally = Object.entries(taps).reduce((s, [n, c]) => s + Number(n) * c, 0)
  useEffect(() => {
    if (Math.abs(num(amount) - tally) > 0.005) setTaps({})
  }, [amount, tally])
  function tap(n: number) {
    const next = { ...taps, [n]: (taps[n] ?? 0) + 1 }
    setTaps(next)
    onChange(String(Object.entries(next).reduce((s, [v, c]) => s + Number(v) * c, 0)))
  }
  const badge = (n: number, coin = false) => taps[n] ? (
    <span className={coin ? 'absolute -top-2 -right-3 grid h-5 min-w-5 place-items-center rounded-full bg-foreground px-1 text-[11px] font-bold text-background tabular-nums' : 'absolute -top-1.5 -right-1.5 grid h-5 min-w-5 place-items-center rounded-full bg-foreground px-1 text-[11px] font-bold text-background tabular-nums'}>×{taps[n]}</span>
  ) : null
  return (
    <div className="flex flex-col gap-2" onPointerDown={(e) => e.preventDefault()}>
      <div role="group" aria-label="Notes handed over" className="grid grid-cols-4 gap-1.5">
        {[...notes].sort((a, b) => a - b).map((n, i) => {
          const tint = `var(--chart-${(i % 6) + 1})`
          return (
            <button key={n} type="button" onClick={() => tap(n)} aria-label={`Add a ${n} note`}
              style={{ background: `color-mix(in srgb, ${tint} 14%, var(--card))`, borderColor: `color-mix(in srgb, ${tint} 55%, transparent)` }}
              className="relative flex h-12 items-center justify-center rounded-[6px] border-[1.5px] shadow-[var(--shadow)] transition-transform active:scale-95">
              <span className="pointer-events-none absolute inset-[3px] rounded-[3px] border border-dashed opacity-40" style={{ borderColor: tint }} />
              <span className="absolute top-1 left-1.5 text-[9px] font-bold opacity-60">Rs</span>
              <b className="font-display text-[17px] tabular-nums">{short(n)}</b>
              {badge(n)}
            </button>
          )
        })}
      </div>
      {coins.length > 0 && (
        <div role="group" aria-label="Coins handed over" className="flex items-center gap-2">
          <span className="text-[11px] font-semibold text-muted-foreground">Coins</span>
          {[...coins].sort((a, b) => b - a).map((n) => (
            <button key={n} type="button" onClick={() => tap(n)} aria-label={`Add a ${n} coin`}
              className="relative grid size-10 place-items-center rounded-full border-[1.5px] border-[color-mix(in_srgb,var(--foreground)_25%,transparent)] bg-muted font-display text-[14px] font-bold tabular-nums shadow-[inset_0_-2px_0_color-mix(in_srgb,var(--foreground)_12%,transparent)] active:scale-95">
              {short(n)}
              {badge(n, true)}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
