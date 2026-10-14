import { useQuery } from '@tanstack/react-query'
import { Plus, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { errMsg } from '@/features/till/api'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { createScript, medicinesQuery, updateScript } from './api'
import { dosesPerDay, supplyDays } from './dosage'
import type { Script, ScriptItemInput } from './types'

type Row = ScriptItemInput & { key: number; label: string; perDayTouched: boolean }
let next = 1
const blank = (): Row => ({ key: next++, drugText: '', dosage: '', quantity: '', perDay: '', label: '', perDayTouched: false })
const today = () => new Date().toLocaleDateString('en-CA')

/** Write or edit a prescription: the doctor, then one row per medicine — a medicine
 * group (any brand fills it), a product or free text; 1-0-1 fills doses a day. */
export function ScriptSheet({ patientId, script, onClose }: { patientId: number; script?: Script; onClose: () => void }) {
  const [doc, setDoc] = useState(script?.prescriberName ?? '')
  const [reg, setReg] = useState(script?.prescriberReg ?? '')
  const [on, setOn] = useState(script?.prescribedOn || today())
  const [repeat, setRepeat] = useState(script?.isRepeat ?? false)
  const [every, setEvery] = useState(String(script?.repeatIntervalDays || ''))
  const [notes, setNotes] = useState(script?.notes ?? '')
  const [rows, setRows] = useState<Row[]>(() => script?.items.length
    ? script.items.map((it) => ({ key: next++, id: it.id, productId: it.productId, groupId: it.groupId, drugText: it.productId || it.groupId ? '' : it.drug,
      label: it.groupName || it.productName, dosage: it.dosage, quantity: it.quantity === '0' ? '' : it.quantity, perDay: it.perDay === '0' ? '' : it.perDay, perDayTouched: true }))
    : [blank()])
  const [busy, setBusy] = useState(false)
  const set = (key: number, p: Partial<Row>) => setRows((rs) => rs.map((r) => (r.key === key ? { ...r, ...p } : r)))

  async function save() {
    const items = rows.filter((r) => r.productId || r.groupId || r.drugText.trim())
    if (!items.length) return void toast.error('Add at least one medicine')
    setBusy(true)
    try {
      const body = { patientId, prescriberName: doc, prescriberReg: reg, prescribedOn: on, notes, isRepeat: repeat, repeatIntervalDays: Number(every) || 0,
        items: items.map(({ id, productId, groupId, drugText, dosage, quantity, perDay }) => ({ id, productId, groupId, drugText, dosage, quantity, perDay })) }
      if (script) await updateScript(script.id, body)
      else await createScript(body)
      toast.success(script ? 'Prescription saved' : 'Prescription added')
      onClose()
    } catch (e) {
      toast.error(errMsg(e, 'Could not save the prescription'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={script ? 'Edit prescription' : 'New prescription'} submit="Save prescription" busy={busy} onSubmit={() => void save()} wide>
      <div className="grid gap-3 md:grid-cols-[1fr_10rem_10rem]">
        <Field label="Doctor"><input aria-label="Doctor" placeholder="Dr …" value={doc} onChange={(e) => setDoc(e.target.value)} className={INPUT} /></Field>
        <Field label="Reg. no"><input aria-label="Doctor's registration" value={reg} onChange={(e) => setReg(e.target.value)} className={INPUT} /></Field>
        <Field label="Date"><input aria-label="Prescribed on" type="date" value={on} onChange={(e) => setOn(e.target.value)} className={INPUT} /></Field>
      </div>
      <div className="grid gap-2">
        <div className="text-[13px] font-semibold text-muted-foreground">Medicines</div>
        {rows.map((r, i) => (
          <ItemRow key={r.key} row={r} n={i + 1} onChange={(p) => set(r.key, p)} onRemove={rows.length > 1 ? () => setRows((rs) => rs.filter((x) => x.key !== r.key)) : undefined} />
        ))}
        <button type="button" onClick={() => setRows((rs) => [...rs, blank()])}
          className="flex items-center gap-1.5 self-start rounded-full border border-dashed border-border px-3 py-1.5 text-[13px] font-semibold text-muted-foreground hover:border-brand hover:text-foreground">
          <Plus className="size-4" /> Another medicine
        </button>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <Toggle label="Repeat prescription" hint="Long-term: reminds every time it runs out." checked={repeat} onChange={setRepeat} />
        {repeat && (
          <Field label="Or remind every (days)" hint="Only for medicines without doses a day.">
            <input aria-label="Remind every days" inputMode="numeric" value={every} onChange={(e) => setEvery(e.target.value.replace(/\D/g, ''))} className={INPUT} />
          </Field>
        )}
      </div>
      <Field label="Notes"><input aria-label="Prescription notes" value={notes} onChange={(e) => setNotes(e.target.value)} className={INPUT} /></Field>
    </FormSheet>
  )
}

function ItemRow({ row, n, onChange, onRemove }: { row: Row; n: number; onChange: (p: Partial<Row>) => void; onRemove?: () => void }) {
  const per = Number(row.perDay) || 0
  const days = supplyDays(Number(row.quantity) || 0, per)
  return (
    <div className="grid gap-2 rounded-[var(--radius-sm)] border border-border bg-card p-2.5 md:grid-cols-[1fr_7rem_6rem_6rem_auto] md:items-center">
      <MedicinePick row={row} n={n} onChange={onChange} />
      <input aria-label={`Dosage ${n}`} placeholder="1-0-1" value={row.dosage} className={INPUT}
        onChange={(e) => {
          const d = e.target.value
          const auto = dosesPerDay(d)
          onChange({ dosage: d, ...(!row.perDayTouched ? { perDay: auto ? String(auto) : '' } : {}) })
        }} />
      <input aria-label={`Tablets ${n}`} placeholder="Qty" inputMode="decimal" value={row.quantity} onChange={(e) => onChange({ quantity: e.target.value })} className={INPUT} />
      <label className="flex items-center gap-1.5 text-[12px] text-faint">
        <input aria-label={`A day ${n}`} inputMode="decimal" value={row.perDay} onChange={(e) => onChange({ perDay: e.target.value, perDayTouched: true })} className={cn(INPUT, 'w-14 px-2 text-center')} />
        a day
      </label>
      <div className="flex items-center justify-between gap-2 text-[12px] text-faint md:justify-end">
        <span>{days > 0 ? `${days} days` : ''}</span>
        {onRemove && <button type="button" aria-label={`Remove medicine ${n}`} onClick={onRemove} className="rounded-full p-1 hover:bg-muted"><X className="size-4" /></button>}
      </div>
    </div>
  )
}

/** A medicine group or product from the catalogue, or what the doctor wrote as typed. */
function MedicinePick({ row, n, onChange }: { row: Row; n: number; onChange: (p: Partial<Row>) => void }) {
  const [open, setOpen] = useState(false)
  const q = useDebounced(row.drugText.trim(), 200)
  const hits = useQuery({ ...medicinesQuery(q), enabled: open && q.length > 1 }).data ?? []
  if (row.productId || row.groupId) {
    return (
      <span className="flex h-[var(--control-h)] items-center gap-2 rounded-[var(--radius-sm)] border border-brand bg-brand-soft px-3 text-[14px] font-semibold">
        <span className="min-w-0 flex-1 truncate">{row.label}</span>
        <small className="font-normal text-faint">{row.groupId ? 'any brand' : 'this product'}</small>
        <button type="button" aria-label={`Change medicine ${n}`} onClick={() => onChange({ productId: null, groupId: null, label: '', drugText: row.label })}><X className="size-4" /></button>
      </span>
    )
  }
  return (
    <div className="relative">
      <input aria-label={`Medicine ${n}`} placeholder="Medicine (group, product or as written)" value={row.drugText} className={INPUT}
        onFocus={() => setOpen(true)} onBlur={() => setTimeout(() => setOpen(false), 150)} onChange={(e) => onChange({ drugText: e.target.value })} />
      {open && hits.length > 0 && (
        <div className="absolute inset-x-0 top-full z-50 mt-1 grid max-h-64 overflow-auto rounded-[var(--radius-sm)] border border-border bg-popover p-1 shadow-[var(--shadow-float)]">
          {hits.map((m) => (
            <button key={`${m.kind}${m.id}`} type="button" onMouseDown={(e) => e.preventDefault()}
              onClick={() => { onChange(m.kind === 'group' ? { groupId: m.id, label: m.name, drugText: '' } : { productId: m.id, label: m.name, drugText: '' }); setOpen(false) }}
              className="flex items-center justify-between gap-2 rounded-md px-2.5 py-2 text-left text-[14px] hover:bg-muted">
              <span className="truncate font-medium">{m.name}</span>
              <small className="text-faint">{m.kind === 'group' ? `group · ${m.members} brand${m.members === 1 ? '' : 's'}` : 'product'}</small>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
