import { useSuspenseQuery } from '@tanstack/react-query'
import { PackageSearch, X } from 'lucide-react'
import { useState } from 'react'
import { INPUT } from '@/components/admin/fields'
import { PageHead, SURFACE } from '@/components/admin/PageHead'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { Designer, type LabelProduct } from '@/features/counter/labels/LabelsTab'
import { configQuery } from '@/features/till/api'
import { cn } from '@/lib/utils'

/** Barcode labels (karots-pos /admin/labels): a product's label, or any custom
 * barcode, previewed as it prints — to the label printer or as a printable sheet. */
export function LabelsPage() {
  const cfg = useSuspenseQuery(configQuery).data
  const [p, setP] = useState<LabelProduct | null>(null)
  return (
    <>
      <PageHead title="Barcode labels" sub="Pick a product, or type any barcode for a custom label." />
      <div className="mx-auto flex max-w-2xl flex-col gap-3">
        <ProductPicker title="Label for a product" onPick={setP} trigger={(open) => (
          <span className="flex">
            <button type="button" onClick={open} className={cn(INPUT, 'flex h-14 flex-1 items-center gap-2.5 text-left', !p && 'text-faint')}>
              <PackageSearch size={18} className="flex-none" /> {p ? p.name : 'From a product — search or scan'}
            </button>
            {p && <button type="button" aria-label="Custom barcode instead" onClick={() => setP(null)} className="-ml-12 grid w-12 place-items-center text-faint hover:text-foreground"><X size={18} /></button>}
          </span>
        )} />
        <div className={cn(SURFACE, 'overflow-hidden')}>
          <Designer key={p?.id ?? 0} p={p} sym={cfg.symbol} defSize={cfg.labelSize} />
        </div>
      </div>
    </>
  )
}
