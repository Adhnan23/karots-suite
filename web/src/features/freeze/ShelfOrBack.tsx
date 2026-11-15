/** A frozen product's write-off: from the shelf (uncounted: added as an extra,
 * then written off — stock unchanged) or from the back (out of the counted stock). */
export type From = '' | 'shelf' | 'back'

export function ShelfOrBack({ value, onChange }: { value: From; onChange: (v: From) => void }) {
  return (
    <div role="radiogroup" aria-label="From the shelf or the back?" className="flex flex-col gap-1.5">
      <span className="text-[13px] font-semibold text-muted-foreground">Frozen: from the shelf or the back?</span>
      <div className="grid grid-cols-2 gap-1.5">
        {([['shelf', 'From the shelf', 'stock stays as counted'], ['back', 'From the back', 'out of the counted stock']] as const).map(([v, label, sub]) => (
          <button key={v} type="button" role="radio" aria-checked={value === v} onClick={() => onChange(v)}
            className="flex flex-col items-start rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-2 text-left aria-checked:border-brand aria-checked:bg-brand aria-checked:text-brand-foreground">
            <b className="text-[13px]">{label}</b>
            <small className="text-[11.5px] opacity-75">{sub}</small>
          </button>
        ))}
      </div>
    </div>
  )
}
