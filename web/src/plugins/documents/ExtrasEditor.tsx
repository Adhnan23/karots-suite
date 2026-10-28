import { Plus, X } from 'lucide-react'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { Seg } from '@/features/till/bits'
import { cn } from '@/lib/utils'
import type { Extra } from './types'

const SUGGEST = ['Urgent', 'Hole punch', 'Corner rounding', 'Stapling']
const PER: ['job' | 'piece', string][] = [['job', 'Per job'], ['piece', 'Per piece']]
const SMALL = cn(INPUT, 'h-10 px-2.5 text-right tabular-nums')

const blank = (name: string): Extra => ({ id: 0, name, price: '', perPiece: false, productId: null, productName: null, qty: '1' })

/** A service's extras (Urgent, Hole punch…): name, price per job or per piece, and any stock each takes. */
export function ExtrasEditor({ extras, onChange }: { extras: Extra[]; onChange: (xs: Extra[]) => void }) {
  const set = (i: number, x: Partial<Extra>) => onChange(extras.map((y, j) => (j === i ? { ...y, ...x } : y)))
  const used = new Set(extras.map((x) => x.name.trim().toLowerCase()))
  return (
    <section aria-label="Extras" className="flex flex-col gap-3 rounded-[var(--radius-sm)] border border-border bg-card p-3">
      <header>
        <h3 className="font-display text-[16px] font-bold">Extras</h3>
        <p className="text-[13px] text-muted-foreground">Ticked at the till beside the job, each on its own receipt line — Urgent, Hole punch, a lanyard for an ID card.</p>
      </header>
      {extras.map((x, i) => (
        <div key={i} aria-label={x.name || 'New extra'} role="group" className="flex flex-col gap-2 border-t border-dashed border-border pt-3 first-of-type:border-0 first-of-type:pt-0">
          <div className="flex flex-wrap items-center gap-2">
            <input aria-label="Extra name" maxLength={40} placeholder="Name" value={x.name} onChange={(e) => set(i, { name: e.target.value })}
              className={cn(INPUT, 'h-10 w-[180px] font-semibold')} />
            <input aria-label={`${x.name || 'Extra'} price`} inputMode="decimal" placeholder="0.00" value={x.price}
              onChange={(e) => set(i, { price: e.target.value.replace(/[^\d.]/g, '') })} className={cn(SMALL, 'w-24')} />
            <Seg label={`${x.name || 'Extra'} charged`} value={x.perPiece ? 'piece' : 'job'} options={PER} onChange={(v) => set(i, { perPiece: v === 'piece' })} />
            <span className="flex-1" />
            <Button type="button" variant="ghost" className="h-9 px-2 text-muted-foreground" aria-label={`Remove ${x.name || 'extra'}`} onClick={() => onChange(extras.filter((_, j) => j !== i))}><X size={15} /></Button>
          </div>
          {x.productId ? (
            <div className="flex items-center gap-2 text-[13.5px]">
              <span className="text-[12px] text-muted-foreground">Takes</span>
              <input aria-label={`${x.name} stock qty`} inputMode="decimal" value={x.qty} onChange={(e) => set(i, { qty: e.target.value.replace(/[^\d.]/g, '') })} className={cn(SMALL, 'w-20')} />
              <span className="min-w-0 flex-1 truncate font-semibold">{x.productName}</span>
              <span className="text-[12px] text-muted-foreground">{x.perPiece ? 'a piece' : 'a job'}</span>
              <Button type="button" variant="ghost" className="h-9 px-2 text-muted-foreground" aria-label={`${x.name} takes no stock`} onClick={() => set(i, { productId: null, productName: null, qty: '1' })}><X size={15} /></Button>
            </div>
          ) : (
            <ProductPicker title={`Stock ${x.name || 'this extra'} takes`} exclude={new Set()}
              onPick={(p) => set(i, { productId: p.id, productName: p.name, qty: '1' })}
              trigger={(open) => <Button type="button" variant="ghost" className="h-9 self-start gap-1.5 px-2.5 text-[13px]" onClick={open}><Plus size={14} /> Takes stock (optional)</Button>} />
          )}
        </div>
      ))}
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="text-[12.5px] font-semibold text-muted-foreground">Add an extra:</span>
        {[...SUGGEST.filter((n) => !used.has(n.toLowerCase())), ''].map((n) => (
          <Button type="button" key={n || 'other'} variant="outline" className="h-8 gap-1 rounded-pill px-3 text-[12.5px]" onClick={() => onChange([...extras, blank(n)])}>
            <Plus size={13} /> {n || 'Other'}
          </Button>
        ))}
      </div>
    </section>
  )
}
