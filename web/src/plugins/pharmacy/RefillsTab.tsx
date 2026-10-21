import { useQuery } from '@tanstack/react-query'
import { Check, MessageCircle, Phone } from 'lucide-react'
import { toast } from 'sonner'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { markCalled, refillsQuery, setScriptActive } from './api'
import { dueBadge, refillMessage, TONE, waNumber } from './labels'
import { useGo } from './nav'
import type { Refill } from './types'

async function act(p: Promise<unknown>, done: string) {
  try {
    await p
    toast.success(done)
  } catch (e) {
    toast.error(errMsg(e))
  }
}

/** Refills due = a call list: who runs out of what, call or WhatsApp them, Called. */
export function RefillsTab() {
  const go = useGo()
  const rows = useQuery(refillsQuery).data
  return (
    <DataList<Refill> label="Refills due" rows={rows ?? []} rowKey={(r) => r.itemId} onRow={(r) => go(`patients/${r.patientId}`)}
      empty={<EmptyState title={!rows ? 'Loading…' : 'Nobody to call'}>{rows && 'A drug shows here 5 days before it runs out (a one-off until a month after).'}</EmptyState>}
      columns={[
        { key: 'patient', header: 'Patient', cell: (r) => r.patient },
        { key: 'drug', header: 'Medicine', cell: (r) => <span>{r.drug}{r.dosage && <small className="ml-1.5 text-faint">{r.dosage}</small>}</span> },
        { key: 'due', header: 'Runs out', cell: (r) => { const b = dueBadge(r.status, r.due); return b && <span className={cn('rounded-pill px-2 py-0.5 text-[11px] font-bold whitespace-nowrap', TONE[b.tone])}>{b.text}</span> } },
        { key: 'doctor', header: 'Doctor', cell: (r) => r.prescriber || '—', wide: true },
        {
          key: 'act', header: '', interactive: true, className: 'text-right',
          cell: (r) => (
            <span className="flex flex-wrap items-center justify-end gap-1.5">
              {r.phone && <a href={`tel:${r.phone}`} className="inline-flex h-8 items-center gap-1 rounded-pill border border-border px-2.5 text-[12px] font-semibold hover:border-brand"><Phone size={13} />{r.phone}</a>}
              {r.phone && (
                <a href={`https://wa.me/${waNumber(r.phone)}?text=${encodeURIComponent(refillMessage(r.drug, r.due))}`} target="_blank" rel="noreferrer"
                  className="inline-flex h-8 items-center gap-1 rounded-pill border border-border px-2.5 text-[12px] font-semibold hover:border-brand"><MessageCircle size={13} />WhatsApp</a>
              )}
              <Button size="sm" variant="secondary" className="h-8 gap-1" onClick={() => void act(markCalled(r.itemId), `${r.patient} — called`)}><Check size={13} /> Called</Button>
              <Button size="sm" variant="ghost" className="h-8" onClick={() => void act(setScriptActive(r.rxId, false), 'Reminders stopped for that prescription')}>Stop</Button>
            </span>
          ),
        },
      ]} />
  )
}
