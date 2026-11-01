import { ScanBarcode } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { errMsg } from '@/features/till/api'
import { Scanner } from '../screens/Scanner'
import { approveCode } from './api'

/** Approve a waiting phone by the code it shows: typed, or its QR scanned with this device's camera. */
export function Approve({ onClose }: { onClose: () => void }) {
  const [code, setCode] = useState('')
  const [scan, setScan] = useState(false)
  const [busy, setBusy] = useState(false)
  async function submit(c = code) {
    setBusy(true)
    try {
      await approveCode(c.trim())
      toast.success('Phone approved')
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Approve a phone" submit="Approve" busy={busy} onSubmit={() => void submit()}>
      <Field label="The code on the phone">
        <div className="flex gap-2">
          <input autoFocus inputMode="numeric" autoComplete="off" maxLength={6} value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
            className={`${INPUT} min-w-0 flex-1 font-mono text-lg tracking-widest`} />
          <button type="button" aria-label="Scan the phone's code" onClick={() => setScan(true)} className={`${INPUT} grid w-12 place-items-center`}><ScanBarcode size={20} /></button>
        </div>
      </Field>
      {scan && <Scanner onCode={(c) => { setScan(false); setCode(c); void submit(c) }} onClose={() => setScan(false)} />}
    </FormSheet>
  )
}
