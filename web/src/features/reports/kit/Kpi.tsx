import { ArrowDownRight, ArrowUpRight } from 'lucide-react'
import type { ReactNode } from 'react'
import { SURFACE } from '@/components/admin/PageHead'
import { num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { pctText } from './numbers'

/** One headline figure. `tone`: in = green when positive, out = red when
 * positive, sign = red when negative. `change` = % vs the previous period
 * (null hides it); `fallIsGood` flips its colours (refunds, expenses). */
export function Kpi({ amount, label, v, sub, tone, change, fallIsGood, className }: {
  amount?: string; label: string; v: ReactNode; sub?: ReactNode; tone?: 'in' | 'out' | 'sign'
  change?: number | null; fallIsGood?: boolean; className?: string
}) {
  const n = num(amount ?? '0')
  const up = (change ?? 0) >= 0
  const good = up !== !!fallIsGood
  return (
    <div className={cn('flex min-w-0 flex-col gap-0.5 px-4 py-3', className)}>
      <small className="text-[12px] font-semibold text-muted-foreground">{label}</small>
      <b className={cn('font-display text-lg tabular-nums md:text-2xl', tone === 'in' && n > 0 && 'text-success', tone === 'out' && n > 0 && 'text-danger', tone === 'sign' && n < 0 && 'text-danger')}>{v}</b>
      {change != null && (
        <small className={cn('inline-flex items-center gap-0.5 text-[12px] font-semibold tabular-nums', good ? 'text-success' : 'text-danger')}>
          {up ? <ArrowUpRight size={13} aria-hidden /> : <ArrowDownRight size={13} aria-hidden />}
          <span><span className="sr-only">{up ? 'up' : 'down'} </span>{pctText(Math.abs(change))}</span>
          <span className="font-normal text-faint">{' '}vs before</span>
        </small>
      )}
      {sub && <small className="text-[12px] text-faint">{sub}</small>}
    </div>
  )
}

/** The strip of headline figures: a row of `cols` on desktop, two-up on phones. */
export function KpiStrip({ label, cols, children }: { label: string; cols: 3 | 4 | 5; children: ReactNode }) {
  return (
    <section aria-label={label} className={cn(SURFACE, 'grid grid-cols-2 divide-border md:divide-x [&>*]:border-border max-md:[&>*]:border-b',
      { 3: 'md:grid-cols-3', 4: 'md:grid-cols-4', 5: 'md:grid-cols-5' }[cols])}>
      {children}
    </section>
  )
}
