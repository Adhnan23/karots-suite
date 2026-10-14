import { useQuery } from '@tanstack/react-query'
import { FilePlus2, Receipt } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { errMsg, searchQuery as tillSearch } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { qtyText } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import type { TillApi } from '../slots'
import { patientQuery, scriptsQuery, updatePatient } from './api'
import { dosesPerDay } from './dosage'
import { dueBadge, qtyOf, TONE } from './labels'
import { salePatient } from './sale'
import { ScriptSheet } from './ScriptSheet'
import type { Patient, Script, ScriptItem } from './types'

/** One patient at the till: allergies first, then each prescription's medicines
 * with when they run out and Add (what is still owed, the best brand in stock). */
export function PatientView({ till, patientId, onChange, onPast, onClose }: {
  till: TillApi; patientId: number; onChange: () => void; onPast: () => void; onClose: () => void
}) {
  const p = useQuery(patientQuery(patientId)).data
  const scripts = useQuery(scriptsQuery(patientId)).data
  const [write, setWrite] = useState(false)
  const [showStopped, setShowStopped] = useState(false)
  if (!p) return <p className="py-6 text-center text-sm text-muted-foreground">Loading…</p>
  const onSale = salePatient(till)?.patientId === p.id

  const putOnSale = () => {
    if (onSale) return
    till.setCustomer(p.customerId)
    till.setSaleMeta('pharmacy', { patientId: p.id, name: p.name })
  }
  async function add(productId: number, name: string, units: number, plugin?: Record<string, unknown>) {
    putOnSale()
    const err = await till.addProduct(productId, { units, plugin })
    if (err) return void toast.error(err)
    toast.success(`Added ${qtyText(units)} × ${name}`)
  }
  const active = scripts?.filter((s) => s.active) ?? []
  const stopped = scripts?.filter((s) => !s.active) ?? []

  return (
    <div className="grid gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="min-w-0 flex-1">
          <div className="truncate text-lg font-bold">{p.name}</div>
          <div className="text-xs text-muted-foreground">{[p.phone, p.nic].filter(Boolean).join(' · ')}{onSale && ' · on this sale'}</div>
        </div>
        <Button type="button" variant="ghost" size="sm" onClick={onChange}>Change patient</Button>
        <Button type="button" variant="secondary" size="sm" onClick={() => setWrite(true)}><FilePlus2 className="size-4" /> Prescription</Button>
      </div>
      <Allergies p={p} />
      {scripts && active.length === 0 && <p className="rounded-[var(--radius)] border border-dashed border-border p-3 text-sm text-muted-foreground">No prescriptions on file — add one, or sell as usual.</p>}
      {active.map((s) => <ScriptCard key={s.id} s={s} onAdd={(it) => it.suggest && void add(it.suggest.productId, it.suggest.name, qtyOf(it), { pharmacy: { rxItemId: it.id, drug: it.drug } })} />)}
      {stopped.length > 0 && (
        <button type="button" className="justify-self-start text-xs font-semibold text-muted-foreground underline" onClick={() => setShowStopped((v) => !v)}>
          {showStopped ? 'Hide' : 'Show'} {stopped.length} stopped prescription{stopped.length === 1 ? '' : 's'}
        </button>
      )}
      {showStopped && stopped.map((s) => <ScriptCard key={s.id} s={s} />)}
      <DaysHelper onAdd={(id, name, n) => void add(id, name, n)} />
      <div className="flex flex-wrap items-center gap-2 border-t border-border pt-3">
        <Button type="button" variant="ghost" size="sm" onClick={onPast}><Receipt className="size-4" /> Record for a past receipt</Button>
        <Button type="button" className="ml-auto" onClick={() => { putOnSale(); toast.success(`${p.name} is on this sale`); onClose() }}>
          {onSale ? 'Done' : 'Use for this sale'}
        </Button>
      </div>
      {write && <ScriptSheet patientId={p.id} onClose={() => setWrite(false)} />}
    </div>
  )
}

function Allergies({ p }: { p: Patient }) {
  const [edit, setEdit] = useState<string | null>(null)
  async function save() {
    try {
      await updatePatient(p.id, { dob: p.dob, sex: p.sex, nic: p.nic, notes: p.notes, allergies: edit ?? '' })
      setEdit(null)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  if (edit !== null) {
    return (
      <form className="flex gap-2" onSubmit={(e) => { e.preventDefault(); void save() }}>
        <input autoFocus aria-label="Allergies" placeholder="Allergies — blank if none" value={edit} onChange={(e) => setEdit(e.target.value)} className={FIELD} />
        <Button type="submit">Save</Button>
      </form>
    )
  }
  return p.allergies ? (
    <button type="button" onClick={() => setEdit(p.allergies)} title="Edit allergies"
      className="rounded-[var(--radius)] border border-danger bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] px-3 py-2 text-left font-bold text-danger">
      ⚠ Allergic to: {p.allergies}
    </button>
  ) : (
    <button type="button" onClick={() => setEdit('')} className="justify-self-start text-xs text-muted-foreground underline">No allergies noted — add</button>
  )
}

function ScriptCard({ s, onAdd }: { s: Script; onAdd?: (it: ScriptItem) => void }) {
  return (
    <div className={cn('rounded-[var(--radius)] border border-border bg-card', !s.active && 'opacity-60')}>
      <div className="flex flex-wrap items-baseline gap-x-2 border-b border-border px-3 py-2 text-xs text-muted-foreground">
        <span className="font-semibold text-foreground">{s.prescriberName || 'Prescription'}</span>
        {s.prescribedOn && <span>{s.prescribedOn}</span>}
        {s.isRepeat && <span className="rounded-full bg-brand-soft px-2 font-bold text-foreground">repeat</span>}
        {!s.active && <span>stopped</span>}
      </div>
      {s.items.map((it) => {
        const badge = dueBadge(it.status, it.nextDue)
        const owed = Number(it.owed)
        return (
          <div key={it.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2 not-last:border-b not-last:border-border">
            <div className="min-w-0 flex-1">
              <div className="truncate font-semibold">{it.drug}</div>
              <div className="text-xs text-muted-foreground">
                {[it.dosage, Number(it.perDay) ? `${qtyText(Number(it.perDay))} a day` : '', Number(it.quantity) ? `${qtyText(Number(it.quantity))} prescribed` : '',
                  Number(it.dispensed) ? `${qtyText(Number(it.dispensed))} given` : ''].filter(Boolean).join(' · ')}
              </div>
            </div>
            {badge && <span className={cn('rounded-full px-2 py-0.5 text-[11px] font-bold whitespace-nowrap', TONE[badge.tone])}>{badge.text}</span>}
            {onAdd && (it.suggest ? (
              <Button type="button" size="sm" onClick={() => onAdd(it)} title={`${it.suggest.name} — ${qtyText(Number(it.suggest.stock))} in stock`}>
                Add {qtyText(qtyOf(it))} · {it.suggest.name}
              </Button>
            ) : <span className="text-[11px] text-faint">{owed > 0 || !Number(it.quantity) ? 'not in the catalogue' : ''}</span>)}
          </div>
        )
      })}
    </div>
  )
}

/** No prescription: "1-0-1 × 2 days = 4" → Add 4 of a product picked here. */
function DaysHelper({ onAdd }: { onAdd: (id: number, name: string, n: number) => void }) {
  const [q, setQ] = useState('')
  const [pick, setPick] = useState<{ id: number; name: string } | null>(null)
  const [dose, setDose] = useState('1-0-1')
  const [days, setDays] = useState('2')
  const dq = useDebounced(q.trim(), 200)
  const hits = useQuery({ ...tillSearch(dq), enabled: !pick && dq.length > 1 }).data?.items.slice(0, 5) ?? []
  const n = Math.ceil(dosesPerDay(dose) * (Number(days) || 0))
  return (
    <details className="rounded-[var(--radius)] border border-border bg-muted/40 px-3 py-2">
      <summary className="cursor-pointer text-sm font-semibold">Days’ worth (no prescription)</summary>
      <div className="mt-2 grid gap-2">
        {pick ? (
          <button type="button" className="justify-self-start rounded-full border border-brand bg-brand-soft px-3 py-1 text-sm font-semibold" onClick={() => setPick(null)}>{pick.name} ×</button>
        ) : (
          <>
            <input aria-label="Medicine for days" placeholder="Medicine" value={q} onChange={(e) => setQ(e.target.value)} className={FIELD} />
            {hits.map((h) => (
              <button key={h.id} type="button" className="rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted" onClick={() => setPick({ id: h.id, name: h.name })}>
                {h.name} <small className="text-faint">· {qtyText(Number(h.stockQty))} in stock</small>
              </button>
            ))}
          </>
        )}
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <input aria-label="Dose" value={dose} onChange={(e) => setDose(e.target.value)} className={cn(FIELD, 'w-24 text-center')} />
          <span>×</span>
          <input aria-label="Days" inputMode="numeric" value={days} onChange={(e) => setDays(e.target.value.replace(/\D/g, ''))} className={cn(FIELD, 'w-16 text-center')} />
          <span>days = <b>{n || '—'}</b></span>
          <Button type="button" size="sm" className="ml-auto" disabled={!pick || n <= 0} onClick={() => pick && onAdd(pick.id, pick.name, n)}>Add {n || ''}</Button>
        </div>
      </div>
    </details>
  )
}
