import { useQueryClient } from '@tanstack/react-query'
import { Wand2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminProductRow, GeneratedBarcode } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'

/** Give a barcode-less item one — typed, scanned or generated. */
export function BarcodeForm({ p, onClose }: { p: AdminProductRow; onClose: () => void }) {
  const qc = useQueryClient()
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      await api(`/admin/products/${p.id}/barcode`, { json: { barcode: code } })
      toast.success(`Barcode ${code} added`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Barcode for ${p.name}`} submit="Save barcode" busy={busy || !code} onSubmit={save}>
      <Field label="Barcode" hint="Scan the one on the pack, or generate an in-store code to print on labels.">
        <div className="flex gap-1.5">
          <input autoFocus name="bc-code" value={code} onChange={(e) => setCode(e.target.value.trim())} placeholder="Type, scan or generate" className={cn(INPUT, 'min-w-0 flex-1 tabular-nums')} />
          <Button type="button" variant="outline" disabled={!!code} className="h-[var(--control-h)] gap-1.5 px-3"
            onClick={async () => { try { setCode((await api<GeneratedBarcode>('/admin/barcode/generate')).barcode) } catch (e) { toast.error(errMsg(e)) } }}><Wand2 size={15} /> Generate</Button>
        </div>
      </Field>
    </FormSheet>
  )
}
