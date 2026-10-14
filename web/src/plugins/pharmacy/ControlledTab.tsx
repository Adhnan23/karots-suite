import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { DataList } from '@/components/admin/DataList'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { qtyText } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { controlledQuery, patientFromCustomer, recordDispense, searchQuery, unrecordedQuery } from './api'
import { useGo } from './nav'
import type { Controlled, Found, Unrecorded } from './types'

const when = (iso: string) => new Date(iso).toLocaleString('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

/** Controlled drugs (marked on the product form) and the sales still needing a patient on record. */
export function ControlledTab() {
  const go = useGo()
  const list = useQuery(controlledQuery).data
  const missing = useQuery(unrecordedQuery).data ?? []
  const [add, setAdd] = useState<Unrecorded | null>(null)
  return (
    <div className="grid gap-4">
      {missing.length > 0 && (
        <section>
          <h2 className="mb-2 font-display text-lg font-bold">Needs a record</h2>
          <DataList<Unrecorded> label="Controlled sales without a record" rows={missing} rowKey={(r) => `${r.saleId}-${r.productId}`}
            columns={[
              { key: 'drug', header: 'Drug', cell: (r) => `${r.name} × ${qtyText(r.qty)}` },
              { key: 'receipt', header: 'Receipt', cell: (r) => <span className="font-mono">{r.receipt}</span> },
              { key: 'at', header: 'Sold', cell: (r) => when(r.at) },
              { key: 'act', header: '', interactive: true, className: 'text-right', cell: (r) => <Button size="sm" onClick={() => setAdd(r)}>Add record</Button> },
            ]} />
        </section>
      )}
      <section>
        <h2 className="mb-2 font-display text-lg font-bold">Register</h2>
        <DataList<Controlled> label="Controlled drugs" rows={list ?? []} rowKey={(r) => r.productId} onRow={(r) => go(`controlled/${r.productId}`)}
          empty={<EmptyState title={!list ? 'Loading…' : 'No controlled drugs'}>{list && 'Mark one on its product form (Pharmacy → Controlled drug). Its register is read from stock movements.'}</EmptyState>}
          columns={[
            { key: 'name', header: 'Drug', cell: (r) => r.name },
            { key: 'schedule', header: 'Schedule', cell: (r) => r.schedule || '—' },
            { key: 'stock', header: 'In stock', num: true, cell: (r) => qtyText(r.stock) },
            { key: 'flags', header: '', cell: (r) => <span className="flex gap-1.5">{r.unrecorded > 0 && <Tag tone="warn">{`${r.unrecorded} to record`}</Tag>}{r.oldRows > 0 && <Tag tone="muted">old register</Tag>}</span> },
          ]} />
      </section>
      {add && <AddRecord sale={add} onClose={() => setAdd(null)} />}
    </div>
  )
}

/** Who a controlled sale went to: pick the patient (or a customer), and the doctor. */
function AddRecord({ sale, onClose }: { sale: Unrecorded; onClose: () => void }) {
  const [q, setQ] = useState('')
  const [who, setWho] = useState<Found | null>(null)
  const [doc, setDoc] = useState('')
  const [busy, setBusy] = useState(false)
  const found = useQuery({ ...searchQuery(useDebounced(q.trim(), 200), true), enabled: !who }).data ?? []
  async function save() {
    if (!who) return void toast.error('Pick the patient')
    setBusy(true)
    try {
      const pid = who.patientId || (await patientFromCustomer(who.customerId)).id
      await recordDispense({ saleId: sale.saleId, patientId: pid, prescriber: doc })
      toast.success(`${sale.receipt} recorded for ${who.name}`)
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Record ${sale.receipt}`} submit="Save record" busy={busy} onSubmit={() => void save()}>
      <p className="text-sm text-muted-foreground">{sale.name} × {qtyText(sale.qty)}, sold {when(sale.at)}.</p>
      <Field label="Patient">
        {who ? (
          <button type="button" className={`${INPUT} flex items-center justify-between text-left`} onClick={() => setWho(null)}>{who.name} <small className="text-faint">change</small></button>
        ) : <input aria-label="Find patient" placeholder="Name, phone or NIC" value={q} onChange={(e) => setQ(e.target.value)} className={INPUT} />}
      </Field>
      {!who && found.map((f) => (
        <button key={f.customerId} type="button" onClick={() => setWho(f)} className="rounded-md px-3 py-2 text-left text-sm hover:bg-muted">
          {f.name} <small className="text-faint">{f.phone}{f.patientId ? '' : ' · customer'}</small>
        </button>
      ))}
      <Field label="Doctor"><input aria-label="Doctor" placeholder="Dr …" value={doc} onChange={(e) => setDoc(e.target.value)} className={INPUT} /></Field>
    </FormSheet>
  )
}
