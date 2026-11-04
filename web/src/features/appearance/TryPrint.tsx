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

const NETWORK = 'network'
/** "192.168.1.50" or "192.168.1.50:9100" as a print target. */
const networkTarget = (addr: string) => {
  const host = addr.trim().replace(/^tcp:\/\//, '')
  return host ? `tcp://${host}` : ''
}

/** Prints the preview above it on a printer picked here: this PC's, a cashier
 * PC's or one on the network. It only tries that printer: Settings stays as it
 * is, nothing is saved with the shop and no drawer opens. */
export function TryPrint({ what, path, blob }: { what: 'receipt' | 'label'; path: string; blob?: Blob }) {
  const list = useQuery(printersQuery).data
  const key = `ks-try-printer-${what}`
  const [printer, setPrinter] = useState(() => remember.get(key))
  const [address, setAddress] = useState(() => remember.get(`${key}-address`))
  const [busy, setBusy] = useState(false)
  const shop = what === 'receipt' ? list?.receiptPrinter : list?.labelPrinter
  const known = printer === NETWORK || list?.printers.some((p) => p.target === printer)
  const target = printer === NETWORK ? networkTarget(address) : printer
  const print = async () => {
    if (!blob) return
    setBusy(true)
    try {
      const sep = path.includes('?') ? '&' : '?'
      await api(target ? `${path}${sep}printer=${encodeURIComponent(target)}` : path, { method: 'POST', blob })
      toast.success(`Sent to the ${what} printer`)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="flex flex-col gap-2 rounded-[var(--radius)] bg-card p-3 shadow-[var(--shadow)]">
      <span className="text-xs font-bold text-muted-foreground">Try it on paper · prints the preview</span>
      <div className="flex flex-wrap items-center gap-2">
        <select aria-label={`Printer for this ${what}`} className={`${INPUT} min-w-0 flex-1 basis-48`} value={printer}
          onChange={(e) => { setPrinter(e.target.value); remember.set(key, e.target.value) }}>
          <option value="">The shop's {what} printer{shop ? ` (${shop})` : ''}</option>
          {printer && list && !known && <option value={printer}>{printer} (not found now)</option>}
          {list?.printers.map((p) => <option key={p.target} value={p.target}>{p.label}</option>)}
          <option value={NETWORK}>A network printer…</option>
        </select>
        {printer === NETWORK && (
          <input aria-label={`Network address for this ${what}`} className={`${INPUT} min-w-0 flex-1 basis-48`} placeholder="192.168.1.50:9100" autoComplete="off" value={address}
            onChange={(e) => { setAddress(e.target.value); remember.set(`${key}-address`, e.target.value) }} />
        )}
        <Button variant="outline" className="h-10 gap-1.5 px-4" disabled={busy || !blob || (printer === NETWORK && !target)} onClick={() => void print()}><Printer size={16} /> Print this</Button>
      </div>
    </div>
  )
}
