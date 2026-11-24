import { useQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { SupplierPick } from '@/features/purchasing/SupplierPick'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { setSupplierField, setSupplierRule, supplierRulesQuery } from './api'
import { CHIP } from './FieldSheet'
import { matchValues } from './supplierRules'
import type { SupplierValue } from './types'

/** Supplier by field (PPF-10…12): one text or dropdown field gives a supplier to
 * products with no supplier of their own — set each value's supplier once while
 * putting products in; a delivery later gives each product its real one. */
export function SupplierRules() {
  const data = useQuery(supplierRulesQuery).data
  const [q, setQ] = useState('')
  const [switching, setSwitching] = useState<number | null>(null)
  if (!data) return <div className={SURFACE}><EmptyState title="Loading…" /></div>
  const chosen = data.fields.find((f) => f.id === data.fieldId)
  const ruled = data.values.filter((v) => v.supplierId).length

  async function choose(id: number, confirm = false) {
    if (id === data!.fieldId) return
    if (ruled > 0 && !confirm) return setSwitching(id)
    setSwitching(null)
    try {
      await setSupplierField(id, confirm)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  async function rule(v: SupplierValue, supplierId: number) {
    try {
      await setSupplierRule(v.value, supplierId)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }

  const rows = matchValues(data.values, q)
  return (
    <>
      <p className="mb-3 max-w-2xl text-sm text-muted-foreground">
        Pick a field like Brand or Company, then the supplier for each of its values. Products with no supplier of their own
        follow it — on Low stock, orders, count sheets and the supplier's page. Their first order or delivery gives them their own.
      </p>
      <div role="group" aria-label="Field that gives the supplier" className="mb-3 flex flex-wrap gap-1.5">
        <button type="button" aria-pressed={data.fieldId === 0} className={CHIP} onClick={() => void choose(0)}>None</button>
        {data.fields.map((f) => <button key={f.id} type="button" aria-pressed={f.id === data.fieldId} className={CHIP} onClick={() => void choose(f.id)}>{f.label}</button>)}
      </div>
      {data.fields.length === 0 ? (
        <div className={SURFACE}><EmptyState title="No text or dropdown field yet">Add one on the Fields tab — Brand, Company, Maker…</EmptyState></div>
      ) : data.fieldId === 0 ? (
        <div className={SURFACE}><EmptyState title="No field chosen">Pick the field whose values tell you the supplier.</EmptyState></div>
      ) : (
        <>
          {!chosen && <p className="mb-3 text-sm text-warning">This field is disabled or can't give a supplier now: its suppliers are kept and work again once it's enabled.</p>}
          <div className="relative mb-3 md:w-72">
            <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
            <input aria-label="Search values" placeholder="Search values" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
          </div>
          <DataList label="Values and their suppliers" rows={rows} rowKey={(v) => v.value.toLowerCase()}
            empty={<EmptyState title={q ? 'Nothing matches' : 'No values yet'}>{!q && 'Fill this field on your products (form, sheet or Fill in) and its values show here.'}</EmptyState>}
            columns={[
              { key: 'value', header: chosen?.label ?? 'Value', cell: (v) => <b>{v.value}</b> },
              { key: 'products', header: 'Products', num: true, cell: (v) => v.products || <span className="text-faint">—</span> },
              { key: 'supplier', header: 'Supplier', interactive: true, className: 'md:w-80', cell: (v) => (
                <span className="block w-60 md:w-auto"><SupplierPick noAdd label={`Supplier for ${v.value}`} value={v.supplierId} onChange={(id) => void rule(v, id)} /></span>
              ) },
            ]} />
        </>
      )}
      <Dialog open={switching !== null} onOpenChange={(o) => { if (!o) setSwitching(null) }}>
        <DialogContent className="gap-3 p-5 sm:max-w-sm">
          <DialogTitle className="font-display text-lg font-bold">Use another field?</DialogTitle>
          <DialogDescription>The {ruled} {ruled === 1 ? 'supplier' : 'suppliers'} set for {chosen?.label ?? 'this field'}'s values will be cleared.</DialogDescription>
          <div className="mt-2 flex justify-end gap-2">
            <Button type="button" variant="ghost" className="h-10 px-4" onClick={() => setSwitching(null)}>Keep it</Button>
            <Button type="button" className="h-10 bg-danger px-4 font-bold text-danger-foreground hover:bg-danger/90" onClick={() => void choose(switching!, true)}>Clear and switch</Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
