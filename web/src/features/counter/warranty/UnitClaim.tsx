import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { ShieldCheck, ShieldX } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { WarrantyClaimRef, WarrantyReplaceRequest } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { configQuery, errMsg } from '@/features/till/api'
import { slipPath } from '@/features/till/print'
import { FIELD } from '@/features/till/ui'
import { cn } from '@/lib/utils'
import { warrantySerialQuery } from '../queries'
import { afterSlip } from '../slip'
import { REASONS } from './reasons'

const date = (iso: string) => new Date(iso).toLocaleDateString([], { dateStyle: 'medium' })

export function Cover({ ok, until, left, months }: { ok: boolean; until: string; left: string; months: number }) {
  const Icon = ok ? ShieldCheck : ShieldX
  return (
    <div className={cn('flex items-center gap-3 rounded-[var(--radius-sm)] px-4 py-3',
      ok ? 'bg-[color-mix(in_srgb,var(--success)_14%,transparent)]' : 'bg-[color-mix(in_srgb,var(--danger)_12%,transparent)]')}>
      <Icon size={28} className={ok ? 'text-success' : 'text-danger'} />
      <span>
        <b className="block">{ok ? `Covered — ${left}` : 'Out of warranty'}</b>
        <small className="text-muted-foreground">{months} months · until {date(until)}</small>
      </span>
    </div>
  )
}

/** One serial: its cover and history, and the swap for a new unit. */
export function UnitClaim({ serial, onSerial }: { serial: string; onSerial: (s: string) => void }) {
  const cfg = useSuspenseQuery(configQuery).data
  const qc = useQueryClient()
  const q = useQuery(warrantySerialQuery(serial))
  const [next, setNext] = useState('')
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  if (q.error) {
    const nf = q.error instanceof ApiError && q.error.status === 404
    return <p className="m-auto max-w-[280px] p-8 text-center text-faint">{nf ? `No unit with serial “${serial}”.` : errMsg(q.error)}</p>
  }
  const u = q.data
  if (!u) return null
  const canSwap = u.status === 'active' && u.underWarranty

  async function swap(e: { preventDefault(): void }) {
    e.preventDefault()
    if (!next.trim()) return void toast.error('Scan the new unit’s serial')
    setBusy(true)
    try {
      const body: WarrantyReplaceRequest = { unitId: u!.id, newSerial: next.trim(), reason }
      const r = await api<WarrantyClaimRef>('/till/warranty/replace', { json: body })
      afterSlip(cfg, `Swapped ${u!.serial} → ${next.trim()}`, slipPath.warranty(r.claimId), false)
      void qc.invalidateQueries({ queryKey: ['till'] })
      onSerial(next.trim())
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-5 overflow-auto p-5">
      <div>
        <h2 className="font-display text-xl font-bold">{u.productName}</h2>
        <p className="font-mono text-[13px] text-muted-foreground">{u.serial}</p>
        <p className="text-[13px] text-muted-foreground">
          Sold {date(u.soldAt)}{u.receiptNo && ` · ${u.receiptNo}`}{u.customerName && ` · ${u.customerName}`}
        </p>
      </div>
      {u.status === 'replaced'
        ? <p className="rounded-[var(--radius-sm)] bg-muted px-4 py-3 text-[13px]">This unit was already swapped — look up its replacement below.</p>
        : <Cover ok={u.underWarranty} until={u.until} left={u.left} months={u.months} />}
      {canSwap && (
        <form onSubmit={swap} autoComplete="off" className="flex flex-col gap-3">
          <label className="flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground">
            New unit's serial
            <input aria-label="New serial" placeholder="Scan the replacement" value={next} onChange={(e) => setNext(e.target.value)} className={cn(FIELD, 'font-mono')} />
          </label>
          <div className="flex flex-wrap gap-1.5">
            {REASONS.map((r) => (
              <button key={r} type="button" aria-pressed={reason === r} onClick={() => setReason(reason === r ? '' : r)}
                className="rounded-full border border-border px-3 py-1 text-[12.5px] font-semibold aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground">{r}</button>
            ))}
          </div>
          <Button type="submit" size="lg" disabled={busy || !next.trim()} className="h-12 text-[15px]">Swap for the new unit</Button>
          <small className="text-muted-foreground">The new unit keeps the same cover (until {date(u.until)}).</small>
        </form>
      )}
      {u.claims.length > 0 && (
        <section>
          <h3 className="mb-1.5 text-[13px] font-bold text-muted-foreground">History</h3>
          <ul className="flex flex-col gap-1">
            {u.claims.map((c) => (
              <li key={c.id} className="flex items-center gap-2 rounded-[var(--radius-sm)] bg-muted px-3 py-2 text-[13px]">
                <span className="flex-1">{date(c.at)} · {c.resolution}{c.reason && ` · ${c.reason}`} · {c.handledBy}</span>
                {c.replacementSerial && (
                  <button type="button" onClick={() => onSerial(c.replacementSerial)} className="font-mono text-[12px] underline-offset-2 hover:underline">{c.replacementSerial}</button>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  )
}
