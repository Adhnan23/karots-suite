import { useQuery } from '@tanstack/react-query'
import { Plus, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { errMsg } from '@/features/till/api'
import { Seg } from '@/features/till/bits'
import { cn } from '@/lib/utils'
import { gridQuery, saveGrid, updateService } from './api'
import { CATEGORY, CELLS, KIND } from './labels'
import type { Category, Grid, GridRow, GridTier, Kind, Service } from './types'

const SUGGEST = ['A4', 'A3', 'A5', 'Legal', 'Photo 4R']
const blankTier = (minQty: number): GridTier => ({ minQty, cells: ['', '', '', ''] })
const SMALL = cn(INPUT, 'h-10 px-2.5 text-right tabular-nums')

function SizeCard({ row, onChange, onRemove }: { row: GridRow; onChange: (r: GridRow) => void; onRemove: () => void }) {
  const tier = (i: number, t: Partial<GridTier>) => onChange({ ...row, tiers: row.tiers.map((x, j) => (j === i ? { ...x, ...t } : x)) })
  const cell = (i: number, c: number, v: string) => {
    const cells = [...row.tiers[i].cells] as GridTier['cells']
    cells[c] = v.replace(/[^\d.]/g, '')
    tier(i, { cells })
  }
  const lastMin = row.tiers.at(-1)?.minQty ?? 0
  return (
    <section aria-label={row.size || 'Any size'} className="flex flex-col gap-3 rounded-[var(--radius-sm)] border border-border bg-card p-3">
      <header className="flex items-center gap-2">
        <input aria-label="Size" maxLength={20} placeholder="Any size" value={row.size} onChange={(e) => onChange({ ...row, size: e.target.value })}
          className={cn(INPUT, 'h-10 max-w-[180px] font-display text-[16px] font-bold')} />
        {!row.size && <small className="text-[12px] text-muted-foreground">used for any size without its own row</small>}
        <span className="flex-1" />
        <Button type="button" variant="ghost" className="h-9 px-2.5 text-muted-foreground" aria-label={`Remove ${row.size || 'Any size'}`} onClick={onRemove}><X size={16} /></Button>
      </header>
      <div className="hidden grid-cols-[104px_repeat(4,minmax(0,1fr))_32px] gap-2 text-[11.5px] font-bold uppercase tracking-wide text-faint sm:grid">
        <span>Price each</span>{CELLS.map((c) => <span key={c} className="text-right">{c}</span>)}<span />
      </div>
      {row.tiers.map((t, i) => (
        <div key={i} className="grid grid-cols-2 items-end gap-2 sm:grid-cols-[104px_repeat(4,minmax(0,1fr))_32px] sm:items-center">
          <label className="col-span-2 flex items-center gap-1.5 text-[13px] text-muted-foreground sm:col-span-1">
            {i === 0 ? <b className="text-foreground">From 1</b> : <>From <input aria-label="From copies" inputMode="numeric" value={t.minQty || ''} onChange={(e) => tier(i, { minQty: Number(e.target.value.replace(/\D/g, '')) || 0 })} className={cn(SMALL, 'w-16 text-center')} /></>}
          </label>
          {CELLS.map((c, ci) => (
            <label key={c} className="flex flex-col gap-0.5 text-[11px] font-semibold text-faint sm:block">
              <span className="sm:sr-only">{c}</span>
              <input aria-label={`${row.size || 'Any size'} ${c} from ${t.minQty}`} inputMode="decimal" placeholder="—" value={t.cells[ci]} onChange={(e) => cell(i, ci, e.target.value)} className={SMALL} />
            </label>
          ))}
          {i > 0 ? <Button type="button" variant="ghost" className="col-span-2 h-9 justify-self-end px-2 text-muted-foreground sm:col-span-1" aria-label="Remove band" onClick={() => onChange({ ...row, tiers: row.tiers.filter((_, j) => j !== i) })}><X size={15} /></Button> : <span className="hidden sm:block" />}
        </div>
      ))}
      <Button type="button" variant="ghost" className="h-9 self-start gap-1.5 px-2.5 text-[13px]" onClick={() => onChange({ ...row, tiers: [...row.tiers, blankTier(lastMin < 100 ? 100 : lastMin * 2)] })}>
        <Plus size={14} /> Bulk price band
      </Button>
      <div className="flex flex-col gap-1.5 border-t border-dashed border-border pt-3">
        <span className="text-[11.5px] font-bold uppercase tracking-wide text-faint">Paper (per sheet — 2-sided uses half)</span>
        {row.paper.map((p, i) => (
          <div key={p.productId} className="flex items-center gap-2 text-[13.5px]">
            <span className="min-w-0 flex-1 truncate font-semibold">{p.name}</span>
            <input aria-label={`${p.name} per sheet`} inputMode="decimal" value={p.perSheet} onChange={(e) => onChange({ ...row, paper: row.paper.map((x, j) => (j === i ? { ...x, perSheet: e.target.value.replace(/[^\d.]/g, '') } : x)) })} className={cn(SMALL, 'w-20')} />
            <span className="text-[12px] text-muted-foreground">a sheet</span>
            <Button type="button" variant="ghost" className="h-9 px-2 text-muted-foreground" aria-label={`Remove ${p.name}`} onClick={() => onChange({ ...row, paper: row.paper.filter((_, j) => j !== i) })}><X size={15} /></Button>
          </div>
        ))}
        <ProductPicker title={`Paper for ${row.size || 'any size'}`} exclude={new Set(row.paper.map((p) => p.productId))}
          onPick={(p) => onChange({ ...row, paper: [...row.paper, { productId: p.id, name: p.name, perSheet: '1' }] })}
          trigger={(open) => <Button type="button" variant="ghost" className="h-9 self-start gap-1.5 px-2.5 text-[13px]" onClick={open}><Plus size={14} /> {row.paper.length ? 'Another paper' : 'Pick the paper'}</Button>} />
      </div>
    </section>
  )
}

function Editor({ s, g, onClose }: { s: Service; g: Grid; onClose: () => void }) {
  const [rows, setRows] = useState<GridRow[]>(g.rows.length ? g.rows : [{ size: 'A4', tiers: [blankTier(1)], paper: [] }])
  const [kind, setKind] = useState<Kind>(s.kind)
  const [category, setCategory] = useState<Category>(s.category)
  const [busy, setBusy] = useState(false)
  const used = new Set(rows.map((r) => r.size.trim().toLowerCase()))
  async function save() {
    setBusy(true)
    try {
      if (kind !== s.kind || category !== s.category) await updateService(s.id, { kind, category })
      if (kind === 'metered') await saveGrid(s.id, { rows: rows.map((r) => ({ ...r, tiers: r.tiers.map((t, i) => (i === 0 ? { ...t, minQty: 1 } : t)) })) })
      toast.success(`${s.name} saved`)
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open wide onOpenChange={(o) => !o && onClose()} title={`Prices & paper — ${s.name}`} submit="Save" busy={busy} onSubmit={() => void save()}>
      <div className="flex flex-wrap gap-4">
        <Field label="Priced"><Seg label="Priced" value={kind} options={KIND} onChange={setKind} /></Field>
        <Field label="Kind of job"><Seg label="Kind of job" value={category} options={CATEGORY} onChange={setCategory} /></Field>
      </div>
      {kind === 'custom' ? (
        <p className="text-[14px] text-muted-foreground">The cashier types the price for each job (a CV, a photo edit, a poster) and can add a note. Pay whoever does the work as an expense tagged to this service, and Service profit shows what's left.</p>
      ) : (
        <>
          <p className="text-[13px] text-muted-foreground">Leave a box empty when you don't offer it. Bulk bands lower the price from that many copies — the till shows the cashier the next band. Toner and ink (per copy) go in the service's recipe.</p>
          {rows.map((r, i) => (
            <SizeCard key={i} row={r} onChange={(x) => setRows(rows.map((y, j) => (j === i ? x : y)))} onRemove={() => setRows(rows.filter((_, j) => j !== i))} />
          ))}
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-[12.5px] font-semibold text-muted-foreground">Add a size:</span>
            {[...SUGGEST.filter((z) => !used.has(z.toLowerCase())), ...(used.has('') ? [] : [''])].map((z) => (
              <Button type="button" key={z || 'any'} variant="outline" className="h-8 gap-1 rounded-pill px-3 text-[12.5px]" onClick={() => setRows([...rows, { size: z, tiers: [blankTier(1)], paper: [] }])}>
                <Plus size={13} /> {z || 'Any size'}
              </Button>
            ))}
          </div>
        </>
      )}
    </FormSheet>
  )
}

/** A print service's grid (size × B&W/colour × sides, with bulk bands) and paper by size. */
export function GridSheet({ s, onClose }: { s: Service; onClose: () => void }) {
  const g = useQuery(gridQuery(s.id)).data
  if (!g) return null
  return <Editor s={s} g={g} onClose={onClose} />
}
