import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Banknote, Coins, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminDenomination, DenominationRequest } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { configQuery, errMsg } from '@/features/till/api'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { denominationsQuery } from '../queries'

function DenominationForm({ d, onClose }: { d?: AdminDenomination; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<DenominationRequest>({ value: d?.value ?? '', isNote: d?.isNote ?? true, active: d?.active ?? true })
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      if (d) await api(`/admin/denominations/${d.id}`, { method: 'PUT', json: f })
      else await api('/admin/denominations', { json: f })
      toast.success(d ? 'Saved' : 'Added')
      void qc.invalidateQueries({ queryKey: ['admin', 'denominations'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={d ? 'Edit denomination' : 'Add a note or coin'} submit={d ? 'Save' : 'Add'} busy={busy || !Number(f.value)} onSubmit={save}>
      <Field label="Value"><input autoFocus aria-label="Value" inputMode="decimal" value={f.value} placeholder="5000" onChange={(e) => setF({ ...f, value: e.target.value.replace(/[^\d.]/g, '') })} className={INPUT} /></Field>
      <Seg label="Note or coin" value={f.isNote ? 'note' : 'coin'} onChange={(v) => setF({ ...f, isNote: v === 'note' })} options={[['note', 'Note'], ['coin', 'Coin']]} />
    </FormSheet>
  )
}

/** Denominations (karots-pos /admin/denominations): the notes and coins the
 * drawer count offers. "In circulation" stays an inline switch on purpose — on
 * a short fixed list that's what people change. */
export function DenominationsPage() {
  const qc = useQueryClient()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const rows = useQuery(denominationsQuery).data
  const [editing, setEditing] = useState<AdminDenomination | 'new' | null>(null)
  const toggle = async (d: AdminDenomination) => {
    try {
      await api(`/admin/denominations/${d.id}`, { method: 'PUT', json: { value: d.value, isNote: d.isNote, active: !d.active } })
      void qc.invalidateQueries({ queryKey: ['admin', 'denominations'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  const remove = async (d: AdminDenomination) => {
    await api(`/admin/denominations/${d.id}`, { method: 'DELETE' })
    toast.success(`${money(sym, d.value)} removed`)
    void qc.invalidateQueries({ queryKey: ['admin', 'denominations'] })
  }
  return (
    <>
      <PageHead title="Denominations" sub="The notes and coins a drawer count offers."
        actions={<Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setEditing('new')}><Plus size={16} /> Add</Button>} />
      <section className={cn(SURFACE, 'md:max-w-2xl')}>
        {rows && rows.length === 0 ? <EmptyState title="None yet">Add the notes and coins in use so cashiers can count the drawer note by note.</EmptyState> : (
          <ul aria-label="Denominations" className="divide-y divide-border">
            {rows?.map((d) => (
              <li key={d.id} className={cn('flex items-center gap-3 px-4 py-2.5', !d.active && 'opacity-60')}>
                <span className="grid size-9 flex-none place-items-center rounded-full bg-brand-soft">{d.isNote ? <Banknote size={17} /> : <Coins size={17} />}</span>
                <span className="min-w-0 flex-1">
                  <b className="tabular-nums">{money(sym, d.value)}</b>
                  <small className="block text-[12px] text-muted-foreground">{d.isNote ? 'Note' : 'Coin'}</small>
                </span>
                <label className="flex cursor-pointer items-center gap-2 text-[13px] text-muted-foreground">
                  <span className="hidden sm:inline">In circulation</span>
                  <input type="checkbox" role="switch" aria-label={`${money(sym, d.value)} in circulation`} checked={d.active} onChange={() => void toggle(d)} className="peer sr-only" />
                  <span aria-hidden className="relative h-6 w-10 flex-none rounded-pill bg-border transition-colors peer-checked:bg-selected peer-focus-visible:ring-2 peer-focus-visible:ring-ring after:absolute after:top-0.5 after:left-0.5 after:h-5 after:w-5 after:rounded-pill after:bg-muted-foreground peer-checked:after:bg-selected-foreground after:shadow after:transition-transform peer-checked:after:translate-x-4" />
                </label>
                <Button variant="ghost" size="icon" aria-label={`Edit ${money(sym, d.value)}`} className="size-9" onClick={() => setEditing(d)}><Pencil size={15} /></Button>
                <ConfirmButton plain className="size-9 p-0" label={<Trash2 size={15} />} title={`Remove ${money(sym, d.value)}?`}
                  body="If a saved drawer count uses it, it stays — take it out of circulation instead." confirm="Remove" onConfirm={() => remove(d)} />
              </li>
            ))}
          </ul>
        )}
      </section>
      {editing && <DenominationForm d={editing === 'new' ? undefined : editing} onClose={() => setEditing(null)} />}
    </>
  )
}
