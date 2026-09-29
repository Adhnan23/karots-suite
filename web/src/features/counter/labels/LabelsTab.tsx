import { useSuspenseQuery } from '@tanstack/react-query'
import { Printer, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { LabelRequest, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { ProductFind } from '../ProductFind'
import { CounterPage } from '../ui'

const SIZES = ['50x25', '50x30', '40x30', '38x25', '100x50']
const COPIES = [1, 6, 12, 24]
const LABEL = 'flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground'
const CHIP = 'rounded-full border border-border px-3 py-1.5 text-[13px] font-semibold text-foreground aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground'

/** Barcode stickers: pick a product (or type a code), check the preview, print. */
export function LabelsTab() {
  const cfg = useSuspenseQuery(configQuery).data
  const [p, setP] = useState<TillProduct | null>(null)
  return (
    <CounterPage flush aside={<Designer key={p?.id ?? 0} p={p} sym={cfg.symbol} defSize={cfg.labelSize} onClear={() => setP(null)} />}>
      <ProductFind title="Labels" sym={cfg.symbol} picked={p?.id ?? null} onPick={setP} filter={(x) => !x.isService} />
    </CounterPage>
  )
}

function Designer({ p, sym, defSize, onClear }: { p: TillProduct | null; sym: string; defSize: string; onClear: () => void }) {
  const [code, setCode] = useState('')
  const [top, setTop] = useState(p?.name ?? '')
  const [bottom, setBottom] = useState(p ? money(sym, p.sellingPrice) : '')
  const [count, setCount] = useState(p ? 12 : 1)
  const [size, setSize] = useState(defSize)
  const [busy, setBusy] = useState(false)
  const shownCode = p ? p.barcode || `SKU${p.id}` : code.trim()
  const [w, h] = size.split('x').map(Number)

  async function print() {
    if (!shownCode) return void toast.error('Pick a product or type a barcode')
    setBusy(true)
    try {
      const body: LabelRequest = { productId: p?.id ?? 0, code: p ? '' : code.trim(), format: '', top, bottom, count, size }
      await api('/till/labels', { json: body })
      toast.success(`${count} label${count === 1 ? '' : 's'} sent to the printer`)
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-5 overflow-auto p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="font-display text-xl font-bold">{p ? p.name : 'Custom label'}</h2>
          <p className="text-[13px] text-muted-foreground">{p ? 'Edit the text if you need to, then print.' : 'Pick a product on the left, or type any barcode.'}</p>
        </div>
        {p && <button type="button" aria-label="Clear product" onClick={onClear} className="text-faint hover:text-foreground"><X size={18} /></button>}
      </div>

      {/* Live sticker preview, drawn to the chosen size's aspect ratio. */}
      <div className="grid place-items-center rounded-[var(--radius-sm)] bg-muted py-6">
        <div aria-label="Label preview" style={{ aspectRatio: `${w} / ${h}` }}
          className="flex w-[min(260px,70%)] flex-col items-center justify-center gap-1 rounded-md bg-paper px-3 py-2 text-center text-ink shadow-[var(--shadow)]">
          {top && <span className="line-clamp-1 text-[12px] font-bold">{top}</span>}
          <span className="h-[38%] w-[86%] bg-[repeating-linear-gradient(90deg,var(--ink)_0_2px,transparent_2px_4px,var(--ink)_4px_5px,transparent_5px_8px)]" />
          <span className="font-mono text-[10px] tracking-wider">{shownCode || '— barcode —'}</span>
          {bottom && <span className="text-[13px] font-extrabold">{bottom}</span>}
        </div>
      </div>

      {!p && (
        <label className={LABEL}>
          Barcode
          <input aria-label="Barcode" placeholder="e.g. 4791234567890" value={code} onChange={(e) => setCode(e.target.value)} className={cn(FIELD, 'font-mono')} />
        </label>
      )}
      <div className="grid grid-cols-2 gap-3">
        <label className={LABEL}>Top line<input aria-label="Top line" value={top} onChange={(e) => setTop(e.target.value)} className={FIELD} /></label>
        <label className={LABEL}>Bottom line<input aria-label="Bottom line" value={bottom} onChange={(e) => setBottom(e.target.value)} placeholder="price, size…" className={FIELD} /></label>
      </div>
      <div className={LABEL}>
        Copies
        <div className="flex items-center gap-1.5">
          {COPIES.map((n) => <button key={n} type="button" aria-pressed={count === n} onClick={() => setCount(n)} className={CHIP}>{n}</button>)}
          <input aria-label="Copies" inputMode="numeric" value={count} onChange={(e) => setCount(Math.min(200, Number(e.target.value.replace(/\D/g, '')) || 1))}
            className={cn(FIELD, 'w-20 text-center tabular-nums')} />
        </div>
      </div>
      <div className={LABEL}>
        Sticker size (mm)
        <div className="flex flex-wrap gap-1.5">
          {[...new Set([defSize, ...SIZES])].map((s) => (
            <button key={s} type="button" aria-pressed={size === s} onClick={() => setSize(s)} className={CHIP}>
              {s.replace('x', ' × ')}{s === defSize && <small className="ml-1 opacity-70">default</small>}
            </button>
          ))}
        </div>
      </div>
      <Button size="lg" onClick={() => void print()} disabled={busy || !shownCode} className="h-12 text-[15px]">
        <Printer size={18} /> Print {count} label{count === 1 ? '' : 's'}
      </Button>
    </div>
  )
}
