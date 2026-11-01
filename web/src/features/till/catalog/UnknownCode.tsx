import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { TillBarcodeRequest, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useDebounced } from '@/lib/useDebounced'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, errMsg, searchQuery } from '../api'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'
import { useAddProduct } from './useAddProduct'

/** A scanned code no product has: one of an item's other barcodes (find the item;
 * the code becomes its barcode too and it goes in the cart), or a new item
 * (Quick item, the code filled in). */
export function UnknownCode({ code }: { code: string }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const ui = useTillUi()
  const add = useAddProduct()
  const qc = useQueryClient()
  const [finding, setFinding] = useState(false)
  const [q, setQ] = useState('')
  const term = useDebounced(q, 250).trim()
  const hits = useQuery({ ...searchQuery(term), enabled: finding && term !== '' }).data?.items ?? []

  async function give(p: TillProduct) {
    try {
      const withCode = await api<TillProduct>(`/till/products/${p.id}/barcodes`, { json: { barcode: code } satisfies TillBarcodeRequest })
      toast.success(`${code} is now a barcode of ${p.name}`)
      ui.close()
      void qc.invalidateQueries({ queryKey: ['till', 'search'] })
      add(withCode)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>No product has this barcode</DialogTitle>
          <DialogDescription>
            <span className="font-mono">{code}</span> — {finding ? 'find the item it belongs to.' : 'is it an item you already sell, with a different barcode, or a new item?'}
          </DialogDescription>
        </DialogHeader>
        {finding ? (
          <div className="flex flex-col gap-2">
            <input autoFocus aria-label="Find the item" placeholder="Name of the item" autoComplete="off" value={q} onChange={(e) => setQ(e.target.value)} className={FIELD} />
            <ul className="flex max-h-[45vh] flex-col gap-1 overflow-auto">
              {hits.map((p) => (
                <li key={p.id}>
                  <button type="button" onClick={() => void give(p)}
                    className="flex w-full items-center gap-3 rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-2 text-left hover:border-brand">
                    <span className="min-w-0 flex-1 truncate font-semibold">{p.name}</span>
                    <span className="text-[13px] text-muted-foreground tabular-nums">{money(sym, p.sellingPrice)}</span>
                  </button>
                </li>
              ))}
              {term !== '' && !hits.length && <li className="py-4 text-center text-sm text-muted-foreground">No item matches “{term}”</li>}
            </ul>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-2">
            <button type="button" autoFocus onClick={() => setFinding(true)}
              className={cn('flex flex-col gap-1 rounded-[var(--radius-sm)] border border-border bg-muted p-3 text-left hover:border-brand')}>
              <b>Existing item</b><span className="text-[12.5px] text-muted-foreground">Same item, a different barcode — add this code to it</span>
            </button>
            <button type="button" onClick={() => ui.open({ kind: 'quickItem', barcode: code, name: '' })}
              className="flex flex-col gap-1 rounded-[var(--radius-sm)] border border-border bg-muted p-3 text-left hover:border-brand">
              <b>New item</b><span className="text-[12.5px] text-muted-foreground">Not in the catalogue yet — sell it as a quick item</span>
            </button>
          </div>
        )}
        <DialogFooter>
          {finding && <Button variant="ghost" onClick={() => setFinding(false)}>Back</Button>}
          <Button variant="ghost" onClick={ui.close}>Cancel</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
