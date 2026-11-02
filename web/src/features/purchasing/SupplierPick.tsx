import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { supplierOptionsQuery } from '@/features/admin/products/queries'
import { cn } from '@/lib/utils'
import { SupplierForm } from './suppliers/SupplierForm'

/** Pick a supplier, or add one on the spot and carry on with the form behind it. */
export function SupplierPick({ label, value, onChange }: { label: string; value: number; onChange: (id: number) => void }) {
  const qc = useQueryClient()
  const suppliers = useQuery(supplierOptionsQuery).data ?? []
  const [adding, setAdding] = useState(false)
  return (
    <div className="flex gap-1.5">
      <select aria-label={label} value={value} onChange={(e) => onChange(Number(e.target.value))} className={cn(INPUT, 'min-w-0 flex-1 appearance-auto')}>
        <option value={0}>None</option>
        {suppliers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
      </select>
      <Button type="button" variant="outline" className="h-[var(--control-h)] gap-1.5 px-3" onClick={() => setAdding(true)}><Plus size={14} /> New</Button>
      {/* The sheet is a portal, but its submit still bubbles to any form this sits in. */}
      {adding && <div onSubmit={(e) => e.stopPropagation()}>
        <SupplierForm onClose={() => setAdding(false)} onSaved={async (r) => {
          await qc.fetchQuery({ ...supplierOptionsQuery, staleTime: 0 })
          onChange(r.id)
        }} />
      </div>}
    </div>
  )
}
