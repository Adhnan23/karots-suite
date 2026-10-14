import { useQuery } from '@tanstack/react-query'
import { Download } from 'lucide-react'
import { useState } from 'react'
import { DataList } from '@/components/admin/DataList'
import { Toggle } from '@/components/admin/fields'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { RangePicker } from '@/features/counter/RangePicker'
import type { Range } from '@/features/counter/range'
import { qtyText } from '@/lib/money'
import { dispensesQuery } from './api'
import { useGo } from './nav'
import type { LogRow } from './types'

const when = (iso: string) => new Date(iso).toLocaleString('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
const items = (r: LogRow) => r.items.map((i) => `${i.drug} × ${qtyText(i.qty)}${i.days ? ` (${i.days} d)` : ''}`).join(', ')

function csv(rows: LogRow[], range: Range) {
  const q = (s: string) => `"${s.replace(/"/g, '""')}"`
  const body = [['When', 'Receipt', 'Patient', 'Doctor', 'Medicine', 'Qty', 'Days', 'Controlled', 'Returned', 'By'],
    ...rows.flatMap((r) => r.items.map((i) => [q(r.at), q(r.receipt), q(r.patient), q(r.prescriber), q(i.drug), i.qty, String(i.days), i.controlled ? 'yes' : '', r.returned ? 'yes' : '', q(r.by)]))]
    .map((r) => r.join(',')).join('\n')
  const a = document.createElement('a')
  a.href = URL.createObjectURL(new Blob([body], { type: 'text/csv' }))
  a.download = `dispensed-${range.from || range.preset || 'log'}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}

/** The dispensing log: who got what, on which receipt, by date. */
export function DispensedTab() {
  const go = useGo()
  const [range, setRange] = useState<Range>({ preset: 'this-month', from: '', to: '' })
  const [ctl, setCtl] = useState(false)
  const rows = useQuery(dispensesQuery(range, ctl)).data
  return (
    <>
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <RangePicker full noLatest value={range} onChange={(r) => setRange({ ...r, preset: r.preset || 'this-month' })} />
        <Toggle label="Controlled only" checked={ctl} onChange={setCtl} className="md:w-56" />
        {!!rows?.length && <Button variant="outline" className="h-10 gap-1.5 px-4 md:ml-auto" onClick={() => csv(rows, range)}><Download size={16} /> CSV</Button>}
      </div>
      <DataList<LogRow> label="Dispensed" rows={rows ?? []} rowKey={(r) => r.id} onRow={(r) => go(`patients/${r.patientId}`)}
        empty={<EmptyState title={!rows ? 'Loading…' : 'Nothing dispensed in this period'} />}
        columns={[
          { key: 'patient', header: 'Patient', cell: (r) => r.patient },
          { key: 'items', header: 'Medicines', cell: items },
          { key: 'at', header: 'When', cell: (r) => when(r.at) },
          { key: 'receipt', header: 'Receipt', cell: (r) => <span className="font-mono text-[13px]">{r.receipt || '—'}</span>, wide: true },
          { key: 'doctor', header: 'Doctor', cell: (r) => r.prescriber || '—', wide: true },
          { key: 'tags', header: '', cell: (r) => <span className="flex gap-1.5">{r.controlled && <Tag tone="warn">Controlled</Tag>}{r.returned && <Tag tone="muted">↩ returned</Tag>}</span> },
        ]} />
    </>
  )
}
