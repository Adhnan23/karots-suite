import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { DataList } from '@/components/admin/DataList'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { configQuery, errMsg } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { MergeInto } from './MergeInto'
import { ProductForm, type Editing } from './ProductForm'
import { reviewQuery } from './queries'

const when = (iso: string) => new Date(iso).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

/** Items quick-added at the till that still need their real details
 *. Finish = open the form; Looks fine = clear the flag. */
export function ReviewPage() {
  const rows = useQuery(reviewQuery).data
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const [editing, setEditing] = useState<Editing | null>(null)
  async function fine(id: number) {
    try {
      await api(`/admin/products/${id}/review-done`, { method: 'POST' })
      toast.success('Marked reviewed')
      void qc.invalidateQueries({ queryKey: ['admin'] })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <>
      <PageHead title="Items to review" sub="Quick-added at the till with only a name and price, or given a pack size there. Give each its real details, or check the size." />
      <DataList label="Items to review" rows={rows ?? []} rowKey={(p) => p.id} onRow={(p) => setEditing({ id: p.id })}
        empty={<EmptyState title={rows ? 'All caught up' : 'Loading…'}>{rows && 'Nothing quick-added is waiting for details.'}</EmptyState>}
        columns={[
          { key: 'name', header: 'Item', cell: (p) => <span className="flex flex-col"><b>{p.name}</b><small className="text-faint">{p.barcode || 'no barcode'}</small>
            {p.tillPack && <small className="font-semibold text-warning">{p.tillPack} set at the till — check the size</small>}</span> },
          { key: 'by', header: 'Added', cell: (p) => <span className="text-muted-foreground">{p.createdBy || '—'} · {when(p.createdAt)}</span> },
          { key: 'price', header: 'Sold at', num: true, cell: (p) => money(sym, p.sellingPrice) },
          { key: 'stock', header: 'On hand', num: true, cell: (p) => `${qtyText(p.stockQty)} ${p.unitAbbr}` },
          { key: 'act', header: '', wide: true, interactive: true, className: 'w-[1%]', cell: (p) => (
            <span className="flex justify-end gap-1.5">
              <ConfirmButton plain label="Looks fine" className="h-9 px-3" title={`Mark ${p.name} as reviewed?`} confirm="Mark reviewed"
                body={p.needsReview ? 'It keeps the name, price and zero cost it was quick-added with. Finish it instead to give it a real cost.' : 'The pack size the cashier typed is right.'} onConfirm={() => fine(p.id)} />
              <MergeInto from={p} onDone={() => undefined} trigger={(open) => (
                <Button variant="outline" className="h-9 px-3" onClick={open}>Same as an existing one</Button>
              )} />
              <Button className="h-9 px-3 font-bold" onClick={() => setEditing({ id: p.id })}>Finish</Button>
            </span>
          ) },
        ]} />
      {editing && <ProductForm key={editing.id} editing={editing} onEdit={setEditing} onClose={() => setEditing(null)} />}
    </>
  )
}
