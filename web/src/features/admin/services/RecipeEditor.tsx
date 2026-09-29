import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Plus, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { Recipe, RecipeComponent, RecipeCost } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { Seg } from '@/features/till/bits'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { recipeQuery } from './queries'
import { recipeCost, usedPerSale } from './recipe'

export function RecipeEditor({ id, onClose }: { id: number; onClose: () => void }) {
  const r = useQuery(recipeQuery(id)).data
  return r ? <Editor r={r} onClose={onClose} /> : null
}

/** What one unit of a service uses (karots-pos recipe editor): stock ingredients
 * per unit sold or by yield, whole units, and non-stock cost lines. */
function Editor({ r, onClose }: { r: Recipe; onClose: () => void }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const [cs, setCs] = useState<RecipeComponent[]>(r.components)
  const [costs, setCosts] = useState<RecipeCost[]>([...r.costs, { label: '', amount: '' }])
  const [busy, setBusy] = useState(false)
  const cost = recipeCost(cs, costs)
  const margin = num(r.sellingPrice) - cost.total
  const setC = (i: number, patch: Partial<RecipeComponent>) => setCs((x) => x.map((c, j) => (j === i ? { ...c, ...patch } : c)))
  const setL = (i: number, patch: Partial<RecipeCost>) => setCosts((x) => {
    const next = x.map((l, j) => (j === i ? { ...l, ...patch } : l))
    return next.at(-1)?.label || next.at(-1)?.amount ? [...next, { label: '', amount: '' }] : next // always one spare row
  })

  async function save() {
    setBusy(true)
    try {
      await api(`/admin/products/${r.productId}/recipe`, { json: { components: cs, costs } })
      toast.success('Recipe saved')
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <FormSheet open wide onOpenChange={(o) => !o && onClose()} title={`Recipe — ${r.name}`} submit="Save recipe" busy={busy} onSubmit={save}>
      <p className="text-[13px] text-muted-foreground">
        What one {r.unitAbbr} of “{r.name}” uses. Selling it deducts these from stock, and their cost becomes the sale's cost.
      </p>
      <ul aria-label="Ingredients" className="flex flex-col gap-2">
        {cs.map((c, i) => (
          <li key={c.productId} className="grid gap-2 rounded-[var(--radius-sm)] border border-border p-2.5 md:grid-cols-[minmax(0,1fr)_110px_auto_auto_auto] md:items-center">
            <div className="flex items-start gap-2 md:contents">
              <span className="min-w-0 flex-1">
                <b className="block truncate text-sm">{c.name}</b>
                <small className="text-faint tabular-nums">{money(sym, c.costPrice)} / {c.unitAbbr} · ≈ {money(sym, usedPerSale(c) * num(c.costPrice))} per {r.unitAbbr}</small>
              </span>
              <button type="button" aria-label={`Remove ${c.name}`} onClick={() => setCs((x) => x.filter((_, j) => j !== i))} className="grid h-8 w-8 flex-none place-items-center rounded-full text-faint hover:bg-muted hover:text-danger md:order-last"><X size={16} /></button>
            </div>
            <input aria-label={`Amount of ${c.name}`} inputMode="decimal" value={c.amount} onChange={(e) => setC(i, { amount: e.target.value.replace(/[^\d.]/g, '') })} className={cn(INPUT, 'h-10 tabular-nums')} />
            <div className="flex flex-wrap items-center gap-3 md:contents">
              <Seg label={`How ${c.name} is used`} value={c.mode as 'per_unit' | 'yield'} onChange={(mode) => setC(i, { mode })}
                options={[['per_unit', `${c.unitAbbr} per ${r.unitAbbr}`], ['yield', 'makes this many']]} />
              <label className="flex items-center gap-1.5 text-[12px] font-semibold text-muted-foreground">
                <input type="checkbox" checked={c.whole} onChange={(e) => setC(i, { whole: e.target.checked })} /> whole units
              </label>
            </div>
          </li>
        ))}
        {cs.length === 0 && <li className="rounded-[var(--radius-sm)] border border-dashed border-border p-4 text-center text-sm text-faint">No ingredients yet — it sells without touching stock.</li>}
      </ul>
      <ProductPicker title="Add an ingredient" exclude={new Set(cs.map((c) => c.productId))}
        onPick={(p) => setCs((x) => [...x, { productId: p.id, name: p.name, unitAbbr: p.unitAbbr, costPrice: p.costPrice, amount: '', mode: 'per_unit', whole: false }])}
        trigger={(open) => <Button type="button" variant="outline" className="h-10 gap-1.5 self-start px-3" onClick={open}><Plus size={15} /> Add an ingredient</Button>} />
      <p className="text-[12px] text-faint">
        <b>{`per ${r.unitAbbr}`}</b> = 18 g of powder in every cup. <b>makes this many</b> = this bag makes 50 cups. Tick <b>whole units</b> for things that can't be part used, like a sheet of paper.
      </p>

      <div className="border-t border-border pt-3">
        <b className="text-sm">Other costs (not stock)</b>
        <p className="mb-2 text-[12px] text-faint">Electricity, gas, a fee — per {r.unitAbbr}. They only help you price it; the real bill is an expense, so they never count against profit.</p>
        <ul className="flex flex-col gap-1.5">
          {costs.map((l, i) => (
            <li key={i} className="grid grid-cols-[minmax(0,1fr)_120px] gap-1.5">
              <input aria-label="Cost name" placeholder="Electricity" value={l.label} onChange={(e) => setL(i, { label: e.target.value })} className={cn(INPUT, 'h-10')} />
              <input aria-label="Cost amount" inputMode="decimal" placeholder="0.00" value={l.amount} onChange={(e) => setL(i, { amount: e.target.value.replace(/[^\d.]/g, '') })} className={cn(INPUT, 'h-10 tabular-nums')} />
            </li>
          ))}
        </ul>
      </div>

      <div className="grid grid-cols-3 gap-2 rounded-[var(--radius-sm)] bg-muted p-3 text-center">
        <Stat label="Stock cost" value={money(sym, cost.stock)} />
        <Stat label="+ other" value={money(sym, cost.other)} />
        <Stat label={`Left of ${money(sym, r.sellingPrice)}`} value={money(sym, margin)} tone={margin < 0 ? 'text-danger' : 'text-success'} />
      </div>
      {cs.some((c) => !(num(c.amount) > 0)) && <p className="text-[12px] text-warning">Give every ingredient an amount ({qtyText(0)} won't save).</p>}
    </FormSheet>
  )
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <span className="flex flex-col">
      <small className="text-[11px] text-faint">{label}</small>
      <b className={cn('font-display tabular-nums', tone)}>{value}</b>
    </span>
  )
}
