import { queryOptions, useQuery } from '@tanstack/react-query'
import { Printer } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { SystemPrinterList } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'

const printersQuery = queryOptions({ queryKey: ['system', 'printers'], queryFn: () => api<SystemPrinterList>('/system/printers') })

const remember = {
  get(key: string) {
    try { return localStorage.getItem(key) ?? '' } catch { return '' }
  },
  set(key: string, value: string) {
    try { localStorage.setItem(key, value) } catch { /* no storage: it just isn't remembered */ }
  },
}

/** Prints the preview above it on a printer picked here, to judge a style on
 * paper. Nothing is saved and no drawer opens. */
export function TryPrint({ what, path, blob }: { what: 'receipt' | 'label'; path: string; blob?: Blob }) {
  const list = useQuery(printersQuery).data
  const key = `ks-try-printer-${what}`
  const [printer, setPrinter] = useState(() => remember.get(key))
  const [busy, setBusy] = useState(false)
  const shop = what === 'receipt' ? list?.receiptPrinter : list?.labelPrinter
  const known = list?.printers.some((p) => p.target === printer)
  const print = async () => {
    if (!blob) return
    setBusy(true)
    try {
      const sep = path.includes('?') ? '&' : '?'
      await api(printer ? `${path}${sep}printer=${encodeURIComponent(printer)}` : path, { method: 'POST', blob })
      toast.success(`Sent to the ${what} printer`)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="flex flex-wrap items-center gap-2">
      <select aria-label={`Printer for this ${what}`} className={`${INPUT} min-w-0 flex-1 basis-48`} value={printer}
        onChange={(e) => { setPrinter(e.target.value); remember.set(key, e.target.value) }}>
        <option value="">The shop's {what} printer{shop ? ` (${shop})` : ''}</option>
        {printer && list && !known && <option value={printer}>{printer} (not found now)</option>}
        {list?.printers.map((p) => <option key={p.target} value={p.target}>{p.label}</option>)}
      </select>
      <Button variant="outline" className="h-10 gap-1.5 px-4" disabled={busy || !blob} onClick={() => void print()}><Printer size={16} /> Print this</Button>
    </div>
  )
}
