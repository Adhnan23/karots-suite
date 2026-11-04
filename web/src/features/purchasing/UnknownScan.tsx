import { useQuery } from '@tanstack/react-query'
import { Link2, Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { TillBarcodeRequest, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { errMsg, searchQuery } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'

/** A scanned code no product has. A new batch often carries a new barcode, so the
 * question comes first: is it something you already have? A cashier's answer works
 * at once and waits on Items to review. */
export function UnknownScan({ code, onLinked, onNew, onCancel }: { code: string; onLinked: (p: TillProduct) => void; onNew: () => void; onCancel: () => void }) {
  const [finding, setFinding] = useState(false)
  const [q, setQ] = useState('')
  const term = useDebounced(q, 250).trim()
  const hits = useQuery({ ...searchQuery(term), enabled: finding && term !== '' }).data?.items.filter((p) => !p.isService) ?? []
  async function link(p: TillProduct) {
    try {
      const withCode = await api<TillProduct>(`/till/products/${p.id}/barcodes`, { json: { barcode: code, source: 'receive' } satisfies TillBarcodeRequest })
      toast.success(`${code} is now a barcode of ${p.name}`)
      onLinked(withCode)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <div className="mb-2 flex flex-col gap-2 rounded-[var(--radius-sm)] border border-brand p-3 text-[13px]">
      <p><b className="font-mono">{code}</b> isn't on any product. {finding ? 'Find the product it belongs to.' : 'Is it a product you already have, with a new barcode?'}</p>
      {finding ? (
        <>
          <input autoFocus aria-label="Find the product" placeholder="Name of the product" autoComplete="off" value={q} onChange={(e) => setQ(e.target.value)} className={cn(FIELD, 'w-full')} />
          <ul className="flex max-h-48 flex-col gap-1 overflow-auto">
            {hits.map((p) => (
              <li key={p.id}><Button type="button" variant="outline" className="h-9 w-full justify-start px-3" onClick={() => void link(p)}><span className="truncate">{p.name}</span></Button></li>
            ))}
            {term !== '' && !hits.length && <li className="py-2 text-center text-muted-foreground">No product matches "{term}"</li>}
          </ul>
        </>
      ) : (
        <div className="flex flex-wrap gap-2">
          <Button type="button" className="h-9 gap-1.5 px-3 font-bold" autoFocus onClick={() => setFinding(true)}><Link2 size={14} /> Same as an existing one</Button>
          <Button type="button" variant="outline" className="h-9 gap-1.5 px-3" onClick={onNew}><Plus size={14} /> It's a new product</Button>
          <Button type="button" variant="ghost" className="h-9 px-3" onClick={onCancel}>Cancel</Button>
        </div>
      )}
    </div>
  )
}
