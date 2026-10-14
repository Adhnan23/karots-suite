import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { DataList } from '@/components/admin/DataList'
import { Field, INPUT } from '@/components/admin/fields'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { RangePicker } from '@/features/counter/RangePicker'
import { rangeQS, type Range } from '@/features/counter/range'
import { configQuery as tillConfigQuery, errMsg } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, reportQuery, saveConfig } from './api'
import type { Config, ReportRow } from './types'

/** Profit per collected job: the pickup sale's job lines − their cost − the repairer. */
export function ReportTab({ onOpen }: { onOpen: (id: number) => void }) {
  const sym = useQuery(tillConfigQuery).data?.symbol ?? 'Rs.'
  const [range, setRange] = useState<Range>({ preset: 'this-month', from: '', to: '' })
  const rep = useQuery(reportQuery(rangeQS(range).slice(1))).data
  const m = (s: string) => money(sym, num(s))
  return (
    <div className="grid gap-3">
      <RangePicker full noLatest value={range} onChange={(r) => setRange({ ...r, preset: r.preset || 'this-month' })} />
      {rep && (
        <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
          {([['Revenue', rep.revenue], ['Parts cost', rep.cost], ['Repairer', rep.repairer], ['Profit', rep.profit]] as const).map(([l, v]) => (
            <div key={l} className="rounded-[var(--radius)] border border-border bg-card px-3 py-2.5">
              <small className="text-muted-foreground">{l}</small>
              <b className={cn('block truncate font-display text-[17px] tabular-nums md:text-xl', l === 'Profit' && (num(v) < 0 ? 'text-danger' : 'text-success'))}>{m(v)}</b>
            </div>
          ))}
        </div>
      )}
      <DataList<ReportRow> label="Repairs profit" rows={rep?.rows ?? []} rowKey={(r) => r.id} onRow={(r) => onOpen(r.id)}
        empty={<EmptyState title={!rep ? 'Loading…' : 'No repairs collected in this period'} />}
        columns={[
          { key: 'ticket', header: 'Ticket', cell: (r) => <b className="font-display tabular-nums">{r.ticketNo}{r.free && <small className="ml-1.5 font-sans font-normal text-faint">free</small>}</b> },
          { key: 'device', header: 'Device', cell: (r) => r.device || '—', wide: true },
          { key: 'rev', header: 'Revenue', num: true, cell: (r) => m(r.revenue) },
          { key: 'cost', header: 'Parts', num: true, cell: (r) => m(r.cost) },
          { key: 'rep', header: 'Repairer', num: true, cell: (r) => m(r.repairer) },
          { key: 'profit', header: 'Profit', num: true, cell: (r) => <b className={cn(num(r.profit) < 0 && 'text-danger')}>{m(r.profit)}</b> },
        ]} />
    </div>
  )
}

/** Shop-wide warranty: one for the whole repair (by days), or each part its own (PC shops). */
export function SettingsTab() {
  const cfg = useQuery(configQuery).data
  if (!cfg) return null
  return <SettingsForm key={`${cfg.warrantyMode}${cfg.defaultWarrantyDays}`} cfg={cfg} />
}

function SettingsForm({ cfg }: { cfg: Config }) {
  const [mode, setMode] = useState(cfg.warrantyMode)
  const [days, setDays] = useState(String(cfg.defaultWarrantyDays))
  async function save() {
    try {
      await saveConfig({ warrantyMode: mode, defaultWarrantyDays: Number(days || 0) })
      toast.success('Repair warranty saved')
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <form className="grid max-w-[520px] gap-3 rounded-[var(--radius)] border border-border bg-card p-4" onSubmit={(e) => { e.preventDefault(); void save() }}>
      <div role="radiogroup" aria-label="Warranty" className="grid gap-2">
        {([['days', 'By days', 'One warranty for the whole repair, set on each job.'], ['parts', 'By part', 'Each part used has its own warranty (PC shops); the job stays covered while any part is.']] as const).map(([v, l, d]) => (
          <button key={v} type="button" role="radio" aria-checked={mode === v} onClick={() => setMode(v)}
            className="rounded-[var(--radius-sm)] border border-border px-3 py-2.5 text-left aria-checked:border-brand aria-checked:bg-brand-soft">
            <b className="block text-sm">{l}</b><small className="text-muted-foreground">{d}</small>
          </button>
        ))}
      </div>
      {mode === 'days' && (
        <Field label="Usual warranty (days)" hint="Filled in on a new job; change it per job.">
          <input inputMode="numeric" value={days} onChange={(e) => setDays(e.target.value.replace(/\D/g, ''))} className={INPUT} />
        </Field>
      )}
      <Button type="submit" className="justify-self-start">Save</Button>
    </form>
  )
}
