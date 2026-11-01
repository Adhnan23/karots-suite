import { useState } from 'react'
import type { CopyProduct, TillCopy, TillProduct } from '@/api/types.gen'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { FIELD } from '@/features/till/ui'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { search } from '../catalog'
import { liveCall, PC_DOWN } from '../quickItem'

/** A scanned code no product has (the PC's prompt): one of an item's other
 * barcodes — find the item, the code becomes its barcode too (on the PC, so live
 * only) and it goes in the cart — or a new item (Quick item, the code filled in). */
export function UnknownCodeSheet({ code, copy, live, onGiven, onNew, onClose }: {
  code: string
  copy: TillCopy
  live: boolean
  onGiven: (p: CopyProduct) => void
  onNew: () => void
  onClose: () => void
}) {
  const [finding, setFinding] = useState(false)
  const [q, setQ] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const hits = q.trim() ? search(copy, q) : []

  async function give(p: CopyProduct) {
    setBusy(true)
    const r = await liveCall<TillProduct>(`/till/products/${p.id}/barcodes`, { barcode: code })
    setBusy(false)
    if ('error' in r) return setError(r.error)
    onGiven(p)
    onClose()
  }

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="bottom" className="max-h-[88dvh] gap-3 p-4">
        <SheetHeader className="p-0">
          <SheetTitle>No product has this barcode</SheetTitle>
          <SheetDescription>
            <span className="font-mono">{code}</span> — {finding ? 'find the item it belongs to.' : 'an item you already sell with a different barcode, or a new item?'}
          </SheetDescription>
        </SheetHeader>
        {finding ? (
          <>
            <input aria-label="Find the item" placeholder="Name of the item" autoComplete="off" value={q} onChange={(e) => { setQ(e.target.value); setError('') }} className={FIELD} />
            {!live && <p role="alert" className="text-sm font-semibold text-destructive">{PC_DOWN}</p>}
            {error && <p role="alert" className="text-sm font-semibold text-destructive">{error}</p>}
            <ul className="-mx-1 min-h-0 overflow-y-auto">
              {hits.map((p) => (
                <li key={p.id}>
                  <button type="button" disabled={busy || !live} onClick={() => void give(p)}
                    className="flex w-full items-center gap-2 border-b border-border px-1 py-2.5 text-left last:border-b-0 disabled:opacity-60">
                    <span className="min-w-0 flex-1 truncate text-sm font-bold">{p.name}</span>
                    <span className="text-[13px] text-muted-foreground tabular-nums">{money(copy.symbol, p.sellingPrice)}</span>
                  </button>
                </li>
              ))}
              {q.trim() !== '' && !hits.length && <li className="py-6 text-center text-sm text-muted-foreground">No item matches “{q}”</li>}
            </ul>
            <button type="button" onClick={() => setFinding(false)} className="rounded-[var(--radius)] bg-muted p-3 text-sm font-bold">Back</button>
          </>
        ) : (
          <div className="grid grid-cols-2 gap-2">
            <button type="button" onClick={() => setFinding(true)}
              className={cn('flex flex-col gap-1 rounded-[var(--radius-sm)] border border-border bg-muted p-3 text-left')}>
              <b>Existing item</b><span className="text-[12.5px] text-muted-foreground">Same item, a different barcode — add this code to it</span>
            </button>
            <button type="button" onClick={onNew} className="flex flex-col gap-1 rounded-[var(--radius-sm)] border border-border bg-muted p-3 text-left">
              <b>New item</b><span className="text-[12.5px] text-muted-foreground">Not in the catalogue yet — sell it as a quick item</span>
            </button>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
