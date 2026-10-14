import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { configQuery, errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { recentSalesQuery, recordDispense } from './api'

/** Forgot to put the patient on the sale? Pick the receipt and it is recorded the same way. */
export function PastReceipt({ patientId, onBack, onDone }: { patientId: number; onBack: () => void; onDone: () => void }) {
  const [q, setQ] = useState('')
  const dq = useDebounced(q.trim(), 200)
  const sales = useQuery(recentSalesQuery(dq)).data
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const [busy, setBusy] = useState(false)
  async function record(saleId: number, receipt: string) {
    setBusy(true)
    try {
      await recordDispense({ saleId, patientId })
      toast.success(`${receipt} recorded`)
      onDone()
    } catch (e) {
      toast.error(errMsg(e, 'Could not record it'))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="grid gap-3">
      <input autoFocus aria-label="Find receipt" placeholder="Receipt no. or customer" value={q} onChange={(e) => setQ(e.target.value)} className={FIELD} />
      <div className="grid max-h-[50vh] gap-1.5 overflow-auto">
        {sales?.map((s) => {
          const why = s.recorded ? `Recorded for ${s.recorded}` : s.medicines === 0 ? 'No medicines' : ''
          return (
            <button key={s.id} type="button" disabled={busy || !!why} onClick={() => void record(s.id, s.receipt)}
              className="grid grid-cols-[auto_1fr_auto] items-center gap-x-3 rounded-[var(--radius)] border border-border bg-card px-3 py-2 text-left enabled:hover:border-brand disabled:opacity-55">
              <span className="font-mono text-sm font-bold">{s.receipt}</span>
              <span className="truncate text-sm">{s.items || '—'}</span>
              <span className="text-sm font-semibold">{money(sym, num(s.total))}</span>
              <span className="text-xs text-muted-foreground">{new Date(s.at).toLocaleString('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}</span>
              <span className="truncate text-xs text-muted-foreground">{s.customer}</span>
              <span className="text-xs font-semibold text-faint">{why}</span>
            </button>
          )
        })}
      </div>
      <Button type="button" variant="ghost" className="justify-self-start" onClick={onBack}>Back</Button>
    </div>
  )
}
