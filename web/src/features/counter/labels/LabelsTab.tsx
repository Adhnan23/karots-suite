import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Printer, Wand2, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AssignBarcodeRequest, DetailRow, GeneratedBarcode, LabelRequest, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { configQuery, errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { ProductFind } from '../ProductFind'
import { CounterPage } from '../ui'
import { LabelPreview } from './LabelPreview'

const SIZES = ['50x25', '50x30', '40x30', '38x25', '100x50']
const FORMATS = ['CODE128', 'EAN13', 'EAN8', 'UPC', 'CODE39']
const COPIES = [1, 6, 12, 24]
const LABEL = 'flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground'
const CHIP = 'rounded-full border border-border px-3 py-1.5 text-[13px] font-semibold text-foreground aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground'

// The top/bottom field choice sticks per device (karots-pos label.slot.*).
const slotDefault = (which: 'top' | 'bottom', fallback: string) => {
  try { return localStorage.getItem(`label.slot.${which}`) || fallback } catch { return fallback }
}
const saveSlot = (which: 'top' | 'bottom', v: string) => {
  try { localStorage.setItem(`label.slot.${which}`, v) } catch { /* private window: just don't remember */ }
}

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
  const qc = useQueryClient()
  const fields = useQuery({
    queryKey: ['till', 'label-fields', p?.id], queryFn: () => api<DetailRow[]>(`/till/products/${p!.id}/label-fields`), enabled: !!p,
  }).data ?? []
  // Product labels pick a FIELD per line; custom labels type free text.
  const options = p ? [
    { key: 'name', label: 'Name', value: p.name },
    { key: 'price', label: 'Price', value: money(sym, p.sellingPrice) },
    ...fields.map((f) => ({ key: `f:${f.label}`, label: f.label, value: f.value })),
    { key: 'none', label: '— none —', value: '' },
  ] : []
  const [topKey, setTopKey] = useState(() => slotDefault('top', 'name'))
  const [bottomKey, setBottomKey] = useState(() => slotDefault('bottom', 'price'))
  const [top, setTop] = useState('')
  const [bottom, setBottom] = useState('')
  const [code, setCode] = useState('')
  const [format, setFormat] = useState('CODE128')
  const [barcode, setBarcode] = useState(p?.barcode ?? '')
  const [count, setCount] = useState(p ? Math.min(200, Math.max(1, Math.floor(num(p.stockQty)))) : 1)
  const [size, setSize] = useState(defSize)
  const [custom, setCustom] = useState(defSize)
  const [busy, setBusy] = useState(false)
  const val = (key: string) => options.find((o) => o.key === key)?.value ?? ''
  const req: LabelRequest = p
    ? { productId: barcode ? p.id : 0, code: barcode, format: 'CODE128', top: val(topKey), bottom: val(bottomKey), count, size: size === 'custom' ? custom : size }
    : { productId: 0, code: code.trim(), format, top, bottom, count, size: size === 'custom' ? custom : size }
  const canPrint = !!req.code

  async function generate() {
    setBusy(true)
    try {
      const g = await api<GeneratedBarcode>('/till/barcode/generate')
      const body: AssignBarcodeRequest = { barcode: g.barcode }
      await api(`/till/products/${p!.id}/barcode`, { json: body })
      setBarcode(g.barcode)
      toast.success(`Barcode ${g.barcode} saved to ${p!.name}`)
      void qc.invalidateQueries({ queryKey: ['till', 'search'] })
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  async function print() {
    setBusy(true)
    try {
      await api('/till/labels', { json: req })
      toast.success(`${count} label${count === 1 ? '' : 's'} sent to the printer`)
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  const slot = (which: 'top' | 'bottom', value: string, set: (v: string) => void) => (
    <select aria-label={which === 'top' ? 'Top line' : 'Bottom line'} value={options.some((o) => o.key === value) ? value : 'none'}
      onChange={(e) => { set(e.target.value); saveSlot(which, e.target.value) }} className={cn(FIELD, 'appearance-auto')}>
      {options.map((o) => <option key={o.key} value={o.key}>{o.label}{o.value && o.key !== 'none' ? ` — ${o.value}` : ''}</option>)}
    </select>
  )

  return (
    <div className="flex flex-col gap-4 overflow-auto p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="font-display text-xl font-bold">{p ? p.name : 'Custom label'}</h2>
          <p className="text-[13px] text-muted-foreground">{p ? 'Choose what goes above and below the barcode.' : 'Pick a product on the left, or type any barcode.'}</p>
        </div>
        {p && <button type="button" aria-label="Clear product" onClick={onClear} className="text-faint hover:text-foreground"><X size={18} /></button>}
      </div>

      <div className="rounded-[var(--radius-sm)] bg-muted py-5"><LabelPreview req={req} /></div>

      {p && !barcode && (
        <div className="flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-warning px-3 py-2 text-[13px]">
          This product has no barcode yet.
          <Button size="sm" variant="secondary" disabled={busy} onClick={() => void generate()}><Wand2 size={14} /> Generate barcode</Button>
        </div>
      )}
      {!p && (
        <label className={LABEL}>
          Barcode
          <div className="flex gap-1.5">
            <input aria-label="Barcode" placeholder="e.g. 4791234567890" value={code} onChange={(e) => setCode(e.target.value)} className={cn(FIELD, 'min-w-0 flex-1 font-mono')} />
            <select aria-label="Barcode format" value={format} onChange={(e) => setFormat(e.target.value)} className={cn(FIELD, 'appearance-auto')}>
              {FORMATS.map((f) => <option key={f} value={f}>{f}</option>)}
            </select>
          </div>
        </label>
      )}
      <div className="grid grid-cols-2 gap-3">
        <label className={LABEL}>Top line
          {p ? slot('top', topKey, setTopKey) : <input aria-label="Top line" placeholder="e.g. item or model" value={top} onChange={(e) => setTop(e.target.value)} className={FIELD} />}
        </label>
        <label className={LABEL}>Bottom line
          {p ? slot('bottom', bottomKey, setBottomKey) : <input aria-label="Bottom line" placeholder="e.g. price" value={bottom} onChange={(e) => setBottom(e.target.value)} className={FIELD} />}
        </label>
      </div>
      <div className={LABEL}>
        Copies {p && <small className="font-normal">set to what's on hand — change it if you're labelling fewer</small>}
        <div className="flex items-center gap-1.5">
          {COPIES.map((n) => <button key={n} type="button" aria-pressed={count === n} onClick={() => setCount(n)} className={CHIP}>{n}</button>)}
          <input aria-label="Copies" inputMode="numeric" value={count} onChange={(e) => setCount(Math.min(200, Number(e.target.value.replace(/\D/g, '')) || 1))}
            className={cn(FIELD, 'w-20 text-center tabular-nums')} />
        </div>
      </div>
      <div className={LABEL}>
        Sticker size (mm)
        <div className="flex flex-wrap items-center gap-1.5">
          {[...new Set([defSize, ...SIZES])].map((s) => (
            <button key={s} type="button" aria-pressed={size === s} onClick={() => setSize(s)} className={CHIP}>
              {s.replace('x', ' × ')}{s === defSize && <small className="ml-1 opacity-70">default</small>}
            </button>
          ))}
          <button type="button" aria-pressed={size === 'custom'} onClick={() => setSize('custom')} className={CHIP}>Custom</button>
          {size === 'custom' && (
            <input aria-label="Custom size" placeholder="W x H (mm)" value={custom} onChange={(e) => setCustom(e.target.value.replace(/[^\dx]/gi, '').toLowerCase())}
              className={cn(FIELD, 'h-9 w-28 text-center tabular-nums')} />
          )}
        </div>
      </div>
      <Button size="lg" onClick={() => void print()} disabled={busy || !canPrint} className="h-12 text-[15px]">
        <Printer size={18} /> {canPrint ? `Print ${count} label${count === 1 ? '' : 's'}` : p ? 'Needs a barcode first' : 'Type a barcode to print'}
      </Button>
    </div>
  )
}
