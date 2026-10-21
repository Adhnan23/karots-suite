import { Check, Search, ShoppingBag } from 'lucide-react'
import { useState } from 'react'

/** A small sample of the shop's surfaces, drawn with the draft palette while it's
 *  edited (the whole page wears it too; this shows the parts the editor page lacks). */
export function PreviewStrip() {
  const [seg, setSeg] = useState('Today')
  return (
    <div className="flex flex-col gap-3" aria-label="Preview">
      <div className="relative overflow-hidden rounded-[var(--radius)] bg-hero p-4 text-hero-foreground shadow-[var(--shadow)]"
        style={{ backgroundImage: 'var(--hero-art)' }}>
        <p className="text-xs font-semibold text-hero-muted">Total</p>
        <p className="font-display text-3xl font-extrabold tabular-nums">Rs. 2,450.00</p>
        <div className="mt-3 flex gap-2">
          <span className="rounded-pill border border-hero-foreground/15 bg-hero-foreground/10 px-3 py-1 text-xs font-semibold">3 items</span>
          <span className="rounded-pill border border-hero-foreground/15 bg-hero-foreground/10 px-3 py-1 text-xs font-semibold">Cash</span>
        </div>
      </div>

      <div className="flex flex-col gap-3 rounded-[var(--radius)] border border-border bg-card p-4 text-card-foreground shadow-[var(--shadow)] [backdrop-filter:var(--glass)]">
        <div className="flex items-center justify-between gap-2">
          <p className="font-display font-bold">Sales today</p>
          <div role="group" aria-label="Sample range" className="flex rounded-pill bg-muted p-0.5 text-xs font-semibold">
            {['Today', 'Week'].map((s) => (
              <button key={s} type="button" aria-pressed={seg === s} onClick={() => setSeg(s)}
                className="rounded-pill px-2.5 py-1 text-muted-foreground transition-colors aria-pressed:bg-selected aria-pressed:text-selected-foreground">{s}</button>
            ))}
          </div>
        </div>
        <div className="flex items-baseline gap-3 text-sm">
          <span className="font-bold text-success">+ Rs. 1,200</span>
          <span className="font-bold text-danger">− Rs. 300</span>
          <span className="font-bold text-warning">2 low</span>
          <span className="text-faint">vs last week</span>
        </div>
        <label className="flex h-10 items-center gap-2 rounded-[var(--radius-sm)] border border-input bg-background/60 px-3 text-sm text-muted-foreground">
          <Search className="size-4" /> <input aria-label="Sample search" placeholder="Search products" className="min-w-0 flex-1 bg-transparent outline-none placeholder:text-faint" />
        </label>
        <div className="flex flex-wrap gap-2">
          <button type="button" className="flex h-10 items-center gap-1.5 rounded-[var(--radius-sm)] bg-brand px-4 text-sm font-bold text-brand-foreground shadow-[var(--glow)] transition-transform hover:-translate-y-(--lift) duration-(--dur-fast)">
            <ShoppingBag className="size-4" /> Complete sale
          </button>
          <button type="button" className="h-10 rounded-[var(--radius-sm)] border border-border bg-card px-4 text-sm font-semibold transition-transform hover:-translate-y-(--lift) duration-(--dur-fast)">Hold</button>
          <span className="flex h-7 items-center gap-1 self-center rounded-pill bg-brand-soft px-2.5 text-xs font-bold"><Check className="size-3.5" /> Paid</span>
          <span className="flex h-7 items-center self-center rounded-pill bg-brand-2 px-2.5 text-xs font-bold text-brand-2-foreground">New</span>
        </div>
      </div>

      <div className="relative grid h-36 place-items-center overflow-hidden rounded-[var(--radius)] border border-border bg-background">
        <div className="absolute inset-0 bg-scrim" />
        <div className="relative w-[80%] rounded-[var(--radius)] bg-popover p-3 text-popover-foreground shadow-[var(--shadow-float)]">
          <p className="text-sm font-bold">Delete this bill?</p>
          <p className="text-xs text-muted-foreground">A dialog over the page.</p>
          <div className="mt-2 flex justify-end gap-1.5">
            <span className="rounded-[var(--radius-sm)] px-2.5 py-1 text-xs font-semibold">Keep</span>
            <span className="rounded-[var(--radius-sm)] bg-danger px-2.5 py-1 text-xs font-bold text-danger-foreground">Delete</span>
          </div>
        </div>
      </div>

      <div className="flex justify-center">
        <div className="flex gap-1 rounded-pill border border-border bg-bar p-1.5 shadow-[var(--shadow-float)] [backdrop-filter:var(--glass)]">
          {['Home', 'Sell', 'Stock'].map((d, i) => (
            <span key={d} className={i === 1 ? 'rounded-pill bg-brand px-3 py-1.5 text-xs font-bold text-brand-foreground' : 'rounded-pill px-3 py-1.5 text-xs font-semibold text-muted-foreground'}>{d}</span>
          ))}
        </div>
      </div>
    </div>
  )
}
