import { Link2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminProductRow, BarcodeRequest } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { ProductPicker } from './ProductPicker'

/** An unknown scan that is really an item the shop already has (new packaging, another
 * supplier): pick it, and the code becomes one of its barcodes instead of a duplicate product. */
export function SameAsExisting({ code, source, onLinked }: { code: string; source: 'intake' | 'receive'; onLinked: (p: AdminProductRow) => void }) {
  const [busy, setBusy] = useState(false)
  async function link(p: AdminProductRow) {
    setBusy(true)
    try {
      await api(`/admin/products/${p.id}/barcodes`, { json: { barcode: code, source } satisfies BarcodeRequest })
      toast.success(`${code} added to ${p.name}`)
      onLinked({ ...p, barcodes: [...p.barcodes, code] })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-[var(--radius-sm)] border border-dashed border-border px-3 py-2 text-[13px]">
      <span className="min-w-[14rem] flex-1 text-muted-foreground"><b className="tabular-nums text-foreground">{code}</b> isn’t in the catalog. Same item as one you already have?</span>
      <ProductPicker title={`Which product is ${code}?`} onPick={(p) => void link(p)}
        trigger={(open) => (
          <Button type="button" variant="outline" className="h-9 gap-1.5 px-3" disabled={busy} onClick={open}><Link2 size={14} /> Same as an existing one</Button>
        )} />
    </div>
  )
}
