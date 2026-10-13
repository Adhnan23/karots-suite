import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, ScanBarcode, Star, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { BarcodeRequest } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { scannedLabel } from '@/lib/barcode'
import { cn } from '@/lib/utils'
import { barcodesQuery } from './queries'

const added = (iso: string) => (iso ? new Date(iso).toLocaleDateString([], { day: 'numeric', month: 'short', year: 'numeric' }) : '')

/** A saved product's other barcodes (the same item under a second code): each says when it
 * was last scanned, can become the main one (labels print it) or be removed. onMain keeps the
 * form's Barcode field in step, so saving the form doesn't put the old main back. */
export function BarcodesBlock({ productId, onMain }: { productId: number; onMain: (code: string) => void }) {
  const qc = useQueryClient()
  const list = useQuery(barcodesQuery(productId)).data ?? []
  const main = list.find((b) => b.main)
  const extras = list.filter((b) => !b.main)
  const [code, setCode] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const refresh = () => qc.invalidateQueries({ queryKey: ['admin'] })
  const base = `/admin/products/${productId}/barcodes`

  async function add() {
    const c = code.trim()
    if (!c || busy) return
    setBusy(true)
    try {
      await api(base, { json: { barcode: c, source: 'form' } satisfies BarcodeRequest })
      setCode('')
      setErr('')
      await refresh()
    } catch (e) {
      setErr(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  async function makeMain(c: string) {
    try {
      await api(`${base}/${encodeURIComponent(c)}/main`, { method: 'POST' })
      onMain(c)
      toast.success(`${c} is now the main barcode`)
      await refresh()
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  const remove = (c: string) => api(`${base}/${encodeURIComponent(c)}`, { method: 'DELETE' }).then(refresh)

  return (
    <section aria-label="Other barcodes" className="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-border p-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3">
        <h3 className="flex items-center gap-1.5 text-[13px] font-semibold text-muted-foreground"><ScanBarcode size={15} /> Other barcodes</h3>
        <small className="text-faint">Same item, another code — every one scans to this product.{main && ` Main barcode ${scannedLabel(main.lastScannedAt)}.`}</small>
      </div>
      {extras.length > 0 && (
        <ul className="flex flex-col divide-y divide-border">
          {extras.map((b) => (
            <li key={b.barcode} className="flex flex-wrap items-center gap-x-1 gap-y-0.5 py-1.5">
              <span className="min-w-0 basis-full sm:flex-1 sm:basis-auto">
                <b className="block font-semibold break-all tabular-nums">{b.barcode}</b>
                <small className="text-faint">added {added(b.addedAt)} · {scannedLabel(b.lastScannedAt)}</small>
              </span>
              <Button type="button" variant="ghost" className="-ml-2.5 h-9 gap-1 px-2.5 text-[13px] sm:ml-0" onClick={() => void makeMain(b.barcode)}>
                <Star size={14} /> Make main
              </Button>
              <ConfirmButton plain className="h-9 px-2.5 text-[13px]" label={<><X size={14} /> Remove</>} title={`Remove ${b.barcode}?`}
                body="Scanning it will no longer find this product." confirm="Remove" onConfirm={() => remove(b.barcode)} />
            </li>
          ))}
        </ul>
      )}
      <div className="flex gap-1.5">
        <input aria-label="Add another barcode" name="prod-extra-code" value={code} placeholder="Scan or type another barcode"
          onChange={(e) => { setCode(e.target.value); setErr('') }}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void add() } }}
          aria-invalid={!!err} className={cn(INPUT, 'min-w-0 flex-1 tabular-nums')} />
        <Button type="button" variant="outline" className="h-[var(--control-h)] gap-1.5 px-3" disabled={!code.trim() || busy} onClick={() => void add()}>
          <Plus size={15} /> Add
        </Button>
      </div>
      {err && <small role="alert" className="text-danger">{err}</small>}
    </section>
  )
}
