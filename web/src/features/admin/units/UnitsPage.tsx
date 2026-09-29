import { queryOptions, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { Unit, UnitRequest } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { DataList } from '@/components/admin/DataList'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'

export const unitsQuery = queryOptions({ queryKey: ['admin', 'units'], queryFn: () => api<Unit[]>('/admin/units') })

type Editing = Unit | 'new' | null

export function UnitsPage() {
  const rows = useQuery(unitsQuery).data
  const [editing, setEditing] = useState<Editing>(null)
  return (
    <>
      <PageHead title="Units" sub="How products are counted: pieces, kilos, litres, packs."
        actions={<Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setEditing('new')}><Plus size={16} /> New unit</Button>} />
      <DataList label="Units" rows={rows ?? []} rowKey={(u) => u.id} onRow={setEditing}
        empty={<EmptyState title={rows ? 'No units yet' : 'Loading…'}>{rows && 'Add the units your products are sold in.'}</EmptyState>}
        columns={[
          { key: 'name', header: 'Name', cell: (u) => <b>{u.name}</b> },
          { key: 'abbr', header: 'Short', cell: (u) => <code className="rounded bg-muted px-1.5 py-0.5 text-[13px]">{u.abbreviation}</code> },
          { key: 'dec', header: 'Quantities', cell: (u) => u.allowDecimal ? 'Fractions allowed (1.25)' : 'Whole numbers only' },
        ]} />
      {editing && <UnitForm key={editing === 'new' ? 'new' : editing.id} unit={editing === 'new' ? null : editing} onClose={() => setEditing(null)} />}
    </>
  )
}

function UnitForm({ unit, onClose }: { unit: Unit | null; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<UnitRequest>({ name: unit?.name ?? '', abbreviation: unit?.abbreviation ?? '', allowDecimal: unit?.allowDecimal ?? false })
  const [busy, setBusy] = useState(false)
  const done = (msg: string) => {
    toast.success(msg)
    void qc.invalidateQueries({ queryKey: ['admin', 'units'] })
    void qc.invalidateQueries({ queryKey: ['till'] })
    onClose()
  }
  async function save() {
    setBusy(true)
    try {
      if (unit) await api(`/admin/units/${unit.id}`, { method: 'PUT', json: f })
      else await api('/admin/units', { json: f })
      done(unit ? 'Unit saved' : 'Unit added')
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={unit ? `Edit ${unit.name}` : 'New unit'} submit={unit ? 'Save' : 'Add unit'} busy={busy} onSubmit={save}
      extra={unit && (
        <ConfirmButton label="Delete" title={`Delete ${unit.name}?`} confirm="Delete unit"
          body="A unit that products still use can't be deleted — move those products first."
          onConfirm={async () => { await api(`/admin/units/${unit.id}`, { method: 'DELETE' }); done('Unit deleted') }} />
      )}>
      <div className="grid grid-cols-[1fr_120px] gap-3">
        <Field label="Name"><input autoFocus required maxLength={30} name="unit-name" value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} className={INPUT} placeholder="Kilogram" /></Field>
        <Field label="Short"><input required maxLength={10} name="unit-abbr" value={f.abbreviation} onChange={(e) => setF({ ...f, abbreviation: e.target.value })} className={INPUT} placeholder="kg" /></Field>
      </div>
      <Toggle label="Allow fractions" hint="Sell 0.75 kg or 1.5 m. Leave off for things counted whole." checked={f.allowDecimal} onChange={(v) => setF({ ...f, allowDecimal: v })} />
    </FormSheet>
  )
}
