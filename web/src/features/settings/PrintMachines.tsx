import { queryOptions, useQuery, useQueryClient } from '@tanstack/react-query'
import { Printer } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { PrintMachine } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Field, INPUT } from '@/components/admin/fields'
import { SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { slipPicture } from '@/features/print/picture'
import { configQuery, errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'

const BASE = '/admin/print-machines'
const machinesQuery = queryOptions({ queryKey: ['admin', 'print-machines'], queryFn: () => api<PrintMachine[]>(BASE), refetchInterval: 10_000 })

/** Cashier PCs whose print helper has called in (Setup puts it there): approve
 * one and pick its printers, and whatever is printed from that PC comes out there. */
export function PrintMachines() {
  const rows = useQuery(machinesQuery).data ?? []
  if (!rows.length) return null
  return (
    <section aria-label="Cashier PCs' printers" className={cn(SURFACE, 'p-4 md:p-5')}>
      <h2 className="font-display text-base font-bold">Cashier PCs' printers</h2>
      <p className="mb-3 text-[13px] text-muted-foreground">Slips, labels and the drawer from these PCs use the printers plugged into them. A printer left on "The shop's" uses the one above.</p>
      <div className="grid gap-3">{rows.map((m) => <Machine key={`${m.id}-${m.receiptPrinter}-${m.labelPrinter}`} m={m} />)}</div>
    </section>
  )
}

function Machine({ m }: { m: PrintMachine }) {
  const qc = useQueryClient()
  const [receipt, setReceipt] = useState(m.receiptPrinter)
  const [label, setLabel] = useState(m.labelPrinter)
  const [busy, setBusy] = useState(false)
  const refresh = () => qc.invalidateQueries({ queryKey: machinesQuery.queryKey })
  const dirty = !m.approved || receipt !== m.receiptPrinter || label !== m.labelPrinter
  const act = async (fn: () => Promise<unknown>, done: string) => {
    setBusy(true)
    try {
      await fn()
      toast.success(done)
      await refresh()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  // Image mode: the same picture the shop's own test slip prints.
  const testSlip = async () => {
    const cfg = await qc.ensureQueryData(configQuery).catch(() => null)
    const pic = cfg?.printAs === 'image' ? await slipPicture('/admin/settings/printer/test').catch(() => undefined) : undefined
    await api(`${BASE}/${m.id}/test`, { method: 'POST', blob: pic?.blob })
  }
  const save = () => act(() => api(`${BASE}/${m.id}`, { method: 'PUT', json: { receiptPrinter: receipt, labelPrinter: label } }), m.approved ? 'Saved' : `${m.name} can print now`)
  const pick = (name: string, value: string, set: (v: string) => void) => (
    <Field label={name}>
      <select aria-label={`${m.name} ${name}`} className={INPUT} value={value} onChange={(e) => set(e.target.value)}>
        <option value="">The shop's</option>
        {[...new Set([...m.printers, ...(value ? [value] : [])])].map((p) => <option key={p} value={p}>{p}{m.printers.includes(p) ? '' : ' (not found now)'}</option>)}
      </select>
    </Field>
  )
  return (
    <div className="grid gap-3 rounded-[var(--radius)] border border-border p-3">
      <div className="flex flex-wrap items-center gap-2">
        <b className="min-w-0 break-words">{m.name || 'A cashier PC'}</b>
        <span className="text-[12px] text-muted-foreground">{m.address}</span>
        <span className={cn('rounded-pill px-2 py-0.5 text-[11px] font-semibold', !m.approved ? 'bg-brand text-brand-foreground' : m.online ? 'bg-muted text-foreground' : 'bg-muted text-muted-foreground')}>
          {!m.approved ? 'Waiting for your OK' : m.online ? 'Connected' : 'Not connected'}
        </span>
      </div>
      {m.printers.length === 0 && <p className="text-[13px] text-muted-foreground">No printers found on this PC. Plug one in and run Setup → Update there.</p>}
      <div className="grid gap-3 sm:grid-cols-2">
        {pick('Receipt printer', receipt, setReceipt)}
        {pick('Label printer', label, setLabel)}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Button className="h-10 px-4 font-bold" disabled={busy || !dirty} onClick={() => void save()}>{m.approved ? 'Save' : 'Approve'}</Button>
        {m.approved && m.receiptPrinter && (
          <Button variant="outline" className="h-10 gap-1.5 px-4" disabled={busy || !m.online} onClick={() => void act(testSlip, 'Test slip sent')}><Printer size={16} /> Test slip</Button>
        )}
        <ConfirmButton plain className="ml-auto h-10 px-4" label="Remove" title={`Remove ${m.name}?`} body="Its printing stops. If its helper is still running, it asks for your OK again." confirm="Remove"
          onConfirm={() => api(`${BASE}/${m.id}`, { method: 'DELETE' }).then(refresh)} />
      </div>
    </div>
  )
}
