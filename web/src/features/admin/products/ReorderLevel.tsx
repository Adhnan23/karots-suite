import type { AdminProduct } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { learning, looksOff, reasonLine } from '@/features/purchasing/reorder/reason'
import { cn } from '@/lib/utils'

export type ReorderValue = { auto: boolean; level: number; start: number }

const digits = (s: string) => Number(s.replace(/\D/g, '')) || 0

/**
 * The product form's Reorder level: Auto (learned every night from sales and
 * deliveries, starting from a guess) or Fixed (the number typed). Shows why an
 * Auto level is what it is, and offers what sales suggest when a Fixed one is off.
 */
export function ReorderLevel({ product, value, onChange }: { product?: AdminProduct; value: ReorderValue; onChange: (v: ReorderValue) => void }) {
  const calc = product?.reorderCalc
  const unit = product?.unitAbbr ?? ''
  const learnt = calc ? learning(calc) : ''
  const mode = (auto: boolean) => onChange(auto
    ? { ...value, auto, start: value.level } // Auto starts from the current number
    : { ...value, auto, level: product?.reorderLevel ?? value.level }) // Fixed keeps the level it had
  return (
    <div className="flex flex-col gap-2 md:col-span-2">
      <div className="flex items-center justify-between gap-2">
        <span className="text-[13px] font-semibold text-muted-foreground">Reorder level</span>
        <div role="radiogroup" aria-label="Reorder level mode" className="flex rounded-[var(--radius-sm)] bg-muted p-[3px]">
          {([[true, 'Auto'], [false, 'Fixed']] as const).map(([auto, label]) => (
            <button key={label} type="button" role="radio" aria-checked={value.auto === auto} onClick={() => mode(auto)}
              className="h-9 min-w-16 rounded-[calc(var(--radius-sm)-3px)] px-3 text-sm font-semibold text-muted-foreground aria-checked:bg-selected aria-checked:text-selected-foreground">
              {label}
            </button>
          ))}
        </div>
      </div>
      {value.auto ? (
        <div className="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-border p-3">
          {product && calc && (
            <>
              <div className="flex items-baseline gap-2">
                <b className="font-display text-3xl tabular-nums">{product.reorderLevel}</b>
                <span className="text-muted-foreground">{unit}{learnt && ' for now'}</span>
              </div>
              {learnt && (
                <div role="img" aria-label={learnt} className="h-2 overflow-hidden rounded-full bg-muted">
                  <div className="h-2 bg-brand" style={{ width: `${calc.learnedPct}%` }} />
                </div>
              )}
              <p className="text-sm text-muted-foreground">{learnt ? `${learnt[0].toUpperCase()}${learnt.slice(1)} — ` : ''}{reasonLine(calc)}. Updated every night.</p>
            </>
          )}
          {!(product && calc) && <p className="text-sm text-muted-foreground">Learned from sales and deliveries, every night.</p>}
          {(!calc || learnt) && (
            <label className="flex flex-col gap-1.5 text-[13px] text-muted-foreground">
              Starting guess <span className="text-faint">(optional — empty learns from sales alone)</span>
              <input inputMode="numeric" aria-label="Starting guess" value={value.start || ''} onChange={(e) => onChange({ ...value, start: digits(e.target.value), level: product ? value.level : digits(e.target.value) })}
                className={INPUT} placeholder="0" />
            </label>
          )}
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          <input inputMode="numeric" aria-label="Reorder level" value={value.level || ''} onChange={(e) => onChange({ ...value, level: digits(e.target.value) })} className={INPUT} placeholder="0" />
          <span className="text-xs text-faint">Flag as running low at or below this.</span>
          {calc && looksOff(value.level, calc) && (
            <div className={cn('flex items-center gap-3 rounded-[var(--radius-sm)] bg-brand-soft px-3 py-2')}>
              <span className="flex-1 text-sm">Sales suggest <b>{calc.calcLevel}</b> — {reasonLine(calc)}.</span>
              <Button type="button" className="h-10 px-3 font-bold" onClick={() => onChange({ ...value, level: calc.calcLevel })}>Use {calc.calcLevel}</Button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
