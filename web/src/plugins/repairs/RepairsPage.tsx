import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { RangePicker } from '@/features/counter/RangePicker'
import { LATEST, rangeQS, type Range } from '@/features/counter/range'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { jobsQuery, type Show } from './api'
import { JobForm } from './JobForm'
import { JobView } from './JobView'
import { Payout } from './Payout'
import { ReportTab, SettingsTab } from './ReportTab'
import type { Row } from './types'
import { device, STATUS, statusTone } from './words'

const CHIP = 'rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground whitespace-nowrap aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

function useGo() {
  const go = useNavigate()
  return (path: string) => void go({ to: '/admin/x/$key/$', params: { key: 'repairs', _splat: path } })
}

/** Apps → Repairs: the jobs (Open · Ready · Collected · Cancelled · All), a job's
 * page (jobs/<id>), a new job, the profit report and the warranty settings. */
export function RepairsPage({ path }: { path: string }) {
  const go = useGo()
  const [tab, id] = path.split('/')
  if (tab === 'jobs' && Number(id)) {
    return (
      <div className="mx-auto max-w-[860px]">
        <JobView id={Number(id)} onBack={() => go('')} onOpen={(n) => go(`jobs/${n}`)} onEdit={() => go(`edit/${id}`)} extra={(j) => <Payout job={j} />} />
      </div>
    )
  }
  if (tab === 'new' || (tab === 'edit' && Number(id))) {
    return (
      <div className="mx-auto max-w-[760px]">
        <PageHead title={tab === 'new' ? 'New repair job' : 'Edit the job'} />
        <JobForm jobId={Number(id) || 0} onSaved={(n) => go(`jobs/${n}`)} onCancel={() => go(id ? `jobs/${id}` : '')} />
      </div>
    )
  }
  const tabs: [string, string][] = [['', 'Jobs'], ['report', 'Profit'], ['settings', 'Warranty']]
  return (
    <>
      <PageHead title="Repairs" sub="Jobs from drop-off to pickup. Deposits, pickup and refunds happen at the till."
        actions={<Button onClick={() => go('new')}><Plus className="size-4" /> New job</Button>} />
      <div className="mb-3 flex flex-wrap items-center gap-1.5">
        {tabs.map(([p, l]) => <button key={p} type="button" aria-pressed={(tab ?? '') === p} className={CHIP} onClick={() => go(p)}>{l}</button>)}
      </div>
      {tab === 'report' ? <ReportTab onOpen={(n) => go(`jobs/${n}`)} /> : tab === 'settings' ? <SettingsTab /> : <JobsTab onOpen={(n) => go(`jobs/${n}`)} />}
    </>
  )
}

const SHOWS: [Show, string][] = [['open', 'Open'], ['ready', 'Ready'], ['collected', 'Collected'], ['cancelled', 'Cancelled'], ['all', 'All']]

function JobsTab({ onOpen }: { onOpen: (id: number) => void }) {
  const [show, setShow] = useState<Show>('open')
  const [q, setQ] = useState('')
  const [range, setRange] = useState<Range>(LATEST)
  const dq = useDebounced(q.trim(), 250)
  const rows = useQuery(jobsQuery(show, dq, rangeQS(range))).data
  return (
    <div className="grid gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[200px] flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-faint" />
          <input aria-label="Find a repair" placeholder="Scan a ticket, or name, phone, device" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'w-full pl-9')} />
        </div>
        <RangePicker value={range} onChange={setRange} />
      </div>
      <div className="flex gap-1.5 overflow-x-auto pb-0.5">
        {SHOWS.map(([s, l]) => <button key={s} type="button" aria-pressed={show === s} className={CHIP} onClick={() => setShow(s)}>{l}</button>)}
      </div>
      <DataList<Row> label="Repair jobs" rows={rows ?? []} rowKey={(r) => r.id} onRow={(r) => onOpen(r.id)}
        empty={<EmptyState title={!rows ? 'Loading…' : dq ? `No repair matches “${dq}”` : 'No repairs here'}>{rows && !dq && 'Jobs are taken in at the till (🔧 Repairs) or with New job.'}</EmptyState>}
        columns={[
          { key: 'ticket', header: 'Ticket', cell: (r) => <b className="font-display tabular-nums">{r.ticketNo}</b> },
          { key: 'device', header: 'Device', cell: (r) => device(r) },
          { key: 'cust', header: 'Customer', cell: (r) => <span>{r.customerName}<small className="ml-1.5 text-faint tabular-nums">{r.customerPhone}</small></span> },
          { key: 'due', header: 'Due', cell: (r) => (r.due || r.dueUrgent) && <span className={cn('rounded-pill px-2 py-0.5 text-[11px] font-bold whitespace-nowrap', r.dueUrgent ? 'bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] text-danger' : 'bg-muted text-muted-foreground')}>{r.due || 'Urgent'}</span> },
          { key: 'status', header: 'Status', cell: (r) => <span className={cn('rounded-pill px-2.5 py-0.5 text-[11px] font-bold whitespace-nowrap', statusTone[r.status])}>{STATUS[r.status]}</span> },
        ]} />
    </div>
  )
}
