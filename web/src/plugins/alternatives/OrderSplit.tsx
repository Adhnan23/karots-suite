import { useState } from 'react'
import { toast } from 'sonner'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { updateSplit } from './api'
import type { Tier } from './types'

const CHIP = 'rounded-pill border border-border bg-card px-3 py-1 text-[12.5px] font-semibold aria-pressed:border-selected aria-pressed:bg-brand-soft'

const run = async (w: Promise<unknown>, msg: string) => {
  try {
    await w
    toast.success(msg)
  } catch (e) {
    toast.error(errMsg(e))
  }
}

/** How a low tier's order divides: by what sells (Automatic) or the shop's own shares (Manual). */
export function OrderSplit({ t }: { t: Tier }) {
  const live = t.members.filter((m) => m.active)
  const [manual, setManual] = useState(!t.splitAuto)
  const [shares, setShares] = useState<Record<number, string>>(() => Object.fromEntries(live.map((m) => [m.productId, m.share == null ? '' : String(m.share)])))
  if (live.length < 2) return null
  const total = live.reduce((n, m) => n + (Number(shares[m.productId]) || 0), 0)
  const changed = t.splitAuto || live.some((m) => (m.share ?? 0) !== (Number(shares[m.productId]) || 0))
  const save = () => void run(updateSplit(t.id, false, Object.fromEntries(live.map((m) => [m.productId, Number(shares[m.productId]) || 0]))), 'Split saved')
  return (
    <div className="flex flex-col gap-2 rounded-[var(--radius-sm)] bg-muted p-2.5 text-[13px]" aria-label={`Order split${t.name ? ` of ${t.name}` : ''}`} role="group">
      <div className="flex flex-wrap items-center gap-1.5">
        <b className="mr-1 font-semibold">When it runs low, order</b>
        <button type="button" className={CHIP} aria-pressed={!manual} onClick={() => { setManual(false); if (!t.splitAuto) void run(updateSplit(t.id, true), 'Split follows sales') }}>Automatic</button>
        <button type="button" className={CHIP} aria-pressed={manual} onClick={() => setManual(true)}>Manual</button>
      </div>
      {!manual && <small className="text-faint">Most of whichever sells fastest, from the last 90 days. An even split if nothing has sold.</small>}
      {manual && (
        <>
          <ul className="grid gap-1.5 sm:grid-cols-2">
            {live.map((m) => (
              <li key={m.productId} className="flex items-center gap-2">
                <span className="min-w-0 flex-1 truncate">{m.name}</span>
                <input aria-label={`Share of ${m.name}`} inputMode="numeric" value={shares[m.productId] ?? ''} onChange={(e) => setShares({ ...shares, [m.productId]: e.target.value.replace(/\D/g, '') })}
                  className={cn(INPUT, 'h-9 w-16 text-right tabular-nums')} /> %
              </li>
            ))}
          </ul>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <small className={cn('tabular-nums', total === 100 ? 'text-faint' : 'font-semibold text-danger')}>{total}% of 100%</small>
            {changed && <Button type="button" className="h-9 px-3 font-bold" disabled={total !== 100} onClick={save}>Save split</Button>}
          </div>
        </>
      )}
      {!t.splitAuto && t.suggest && (
        <div className="flex flex-wrap items-center justify-between gap-2 rounded-[var(--radius-sm)] bg-brand-soft p-2">
          <span><b>{t.suggest.name}</b> is selling faster than its share. {live.map((m) => `${m.name} ${t.suggest?.shares[m.productId] ?? 0}%`).join(' · ')}</span>
          <Button type="button" variant="outline" className="h-9 px-3" onClick={() => { const next = t.suggest?.shares ?? {}; setShares(Object.fromEntries(Object.entries(next).map(([k, v]) => [k, String(v)]))); void run(updateSplit(t.id, false, next), 'Split updated') }}>Use this</Button>
        </div>
      )}
    </div>
  )
}
