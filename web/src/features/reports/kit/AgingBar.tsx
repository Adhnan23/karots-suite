import { SURFACE } from '@/components/admin/PageHead'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { pctText } from './numbers'

const LABELS = ['0–30 days', '31–60 days', '61–90 days', 'Over 90 days']
const TONES = ['var(--chart-2)', 'var(--chart-1)', 'var(--warning)', 'var(--danger)']

/** How old the money owed is (karots-pos agingBar): one bar split by age, with
 * each part's amount and share named underneath (never by colour alone). */
export function AgingBar({ buckets, total, sym }: { buckets: string[]; total: string; sym: string }) {
  const t = num(total)
  return (
    <section aria-label="How old it is" className={cn(SURFACE, 'flex flex-col gap-3 p-4')}>
      <h2 className="font-display text-base font-bold">How old it is</h2>
      <div className="flex h-3 overflow-hidden rounded-full bg-muted">
        {t > 0 && buckets.map((b, i) => num(b) > 0 && <span key={i} style={{ width: `${(num(b) / t) * 100}%`, background: TONES[i] }} className="border-r-2 border-card last:border-0" />)}
      </div>
      <ul className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {buckets.map((b, i) => (
          <li key={i} className="flex flex-col">
            <span className="flex items-center gap-1.5 text-[12px] font-semibold text-muted-foreground"><span className="h-2.5 w-2.5 rounded-sm" style={{ background: TONES[i] }} />{LABELS[i]}</span>
            <b className="font-display tabular-nums">{money(sym, b)}</b>
            <small className="text-[12px] text-faint">{t > 0 ? pctText((num(b) / t) * 100) : '—'}</small>
          </li>
        ))}
      </ul>
    </section>
  )
}
