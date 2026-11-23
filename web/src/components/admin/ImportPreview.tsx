import { useState } from 'react'
import type { EntityImportResult } from '@/api/types.gen'

const LABEL: Record<string, string> = {
  name: 'Name', name_local: 'Local name', barcode: 'Barcode', extra_barcodes: 'Extra barcodes', category: 'Category',
  unit: 'Unit', cost_price: 'Cost', selling_price: 'Selling price', wholesale_price: 'Wholesale', tax_rate: 'Tax %',
  reorder_level: 'Reorder level', supplier: 'Supplier', track_serial: 'Serials', warranty_months: 'Warranty (months)', active: 'Active',
}
const blank = (v: string) => (v === '' ? '—' : v)

/** A products sheet's preview (docs/rules/stock.md IMP-4): what the import will
 *  do, product by product, old → new. Nothing has changed yet. */
export function ImportPreview({ r }: { r: EntityImportResult }) {
  const [q, setQ] = useState('')
  const changes = r.changes ?? []
  const find = q.trim().toLowerCase()
  const shown = changes.filter((c) => !find || c.product.toLowerCase().includes(find) || c.fields.some((f) => `${f.from} ${f.to}`.toLowerCase().includes(find)))
  const edits = shown.filter((c) => !c.new)
  const added = shown.filter((c) => c.new)
  return (
    <section aria-label="Import preview" className="grid min-w-0 gap-2 rounded-[calc(var(--radius)-4px)] border border-border bg-muted/40 p-3 text-sm">
      <p className="text-[13px] text-muted-foreground">Nothing has changed yet. This is what <b className="text-foreground">Import</b> will do:</p>
      <div className="flex flex-wrap gap-x-4 gap-y-1 font-semibold">
        <span>{r.updated} to update</span>
        <span className="text-success">{r.created} new</span>
        {r.skipped > 0 && <span className="text-danger">{r.skipped} skipped</span>}
      </div>
      {changes.length > 0 && (
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Find a product or a value…" aria-label="Find in the preview"
          className="h-9 w-full min-w-0 rounded-[calc(var(--radius)-4px)] border border-border bg-background px-3 text-sm" />
      )}
      {edits.length > 0 && (
        <ul className="grid max-h-72 min-w-0 gap-2 overflow-y-auto">
          {edits.map((c) => (
            <li key={`${c.line}-${c.product}`} className="min-w-0 border-b border-border pb-2 last:border-0">
              <b className="break-words">{c.product}</b> <span className="text-xs tabular-nums text-faint">line {c.line}</span>
              <dl className="mt-1 grid gap-0.5 text-[13px]">
                {c.fields.map((f) => (
                  <div key={f.field} className="grid min-w-0 grid-cols-[minmax(0,7rem)_minmax(0,1fr)] gap-2">
                    <dt className="truncate text-muted-foreground">{LABEL[f.field] ?? f.field}</dt>
                    <dd className="min-w-0 break-words"><span className="text-muted-foreground line-through decoration-border">{blank(f.from)}</span> → <span>{blank(f.to)}</span></dd>
                  </div>
                ))}
              </dl>
            </li>
          ))}
        </ul>
      )}
      {added.length > 0 && (
        <div className="min-w-0">
          <p className="font-medium">New products</p>
          <p className="max-h-28 overflow-y-auto break-words text-[13px] text-muted-foreground">{added.map((c) => c.product).join(' · ')}</p>
        </div>
      )}
      {r.notes.length > 0 && (
        <ul aria-label="Notes" className="grid max-h-28 gap-0.5 overflow-auto break-words text-[12px] text-muted-foreground">{r.notes.map((n) => <li key={n}>{n}</li>)}</ul>
      )}
      {r.errors.length > 0 && (
        <ul className="grid max-h-32 gap-0.5 overflow-auto text-[13px]">
          {r.errors.map((e) => <li key={e.line}><b className="tabular-nums">Line {e.line}</b> <span className="text-muted-foreground">{e.message} — skipped</span></li>)}
        </ul>
      )}
    </section>
  )
}
